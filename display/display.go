package display

/*
#cgo CFLAGS: -I${SRCDIR}/../protocol
#cgo LDFLAGS: -lwayland-client -lm
#include "wl-gammarelay.h"
#include <wayland-client.h>
#include "../protocol/wlr-gamma-control-unstable-v1-protocol.c"
*/
import "C"
import (
	"context"
	"fmt"
	"log"
)

// Display is a wrapper around a Wayland display.
type Display struct {
	state *C.wl_gammarelay_t

	setColorCh chan setColorRequest
	teardownCh chan struct{}
}

type setColorRequest struct {
	params ColorParams
	errCh  chan<- error
}

// NewDisplay connects to Wayland server and gets a hold of the display.
func New() (*Display, error) {
	state := C.wl_gammarelay_init()
	if state == nil {
		panic("failed to initialize gammarelay")
	}

	if state.display == nil {
		return nil, fmt.Errorf("got a nil display")
	}

	pollNow := make(chan struct{}, 1)
	pollResult := make(chan struct{}, 1)

	// defer close(pollNow) // FIXME

	d := &Display{
		state:      state,
		setColorCh: make(chan setColorRequest),
		teardownCh: make(chan struct{}),
	}

	go func() {
		defer log.Println("poll goroutine done")

		// TODO figure out how to terminate this goroutine on close.

		for range pollNow {
			log.Println("poll goroutine done")
			ret := C.wl_gammarelay_poll(state)
			log.Println("poll ret", ret)

			if ret < 0 {
				log.Println("pollResult close")
				close(pollResult)
				return
			}

			if ret == 0 {
				// timeout
				continue
			}

			select {
			case pollResult <- struct{}{}:
			default:
			}
		}
	}()

	lastColor := ColorParams{
		Temperature: 6500,
		Brightness:  1.0,
	}

	lastNumOutputs := 0

	setColor := func(params ColorParams) error {
		gamma := [3]C.float{
			C.float(1),
			C.float(1),
			C.float(1),
		}

		ret := C.wl_gammarelay_color_set(d.state, C.color_setting_t{
			temperature: C.int(params.Temperature),
			gamma:       gamma,
			brightness:  C.float(params.Brightness),
		})

		if ret != 0 {
			return fmt.Errorf("failed to set temperature: %d", ret)
		}

		lastColor = params

		return nil
	}

	handleSetColor := func(req setColorRequest) {
		defer close(req.errCh)

		err := setColor(req.params)
		if err != nil {
			req.errCh <- fmt.Errorf("failed to set temperature: %d", err)
			return
		}
	}

	handlePoll := func() int {
		return int(C.wl_display_dispatch(state.display))
	}

	go func() {
		for {
			C.wl_display_dispatch_pending(state.display)
			C.wl_display_flush(state.display)

			numOutputs := int(C.wl_gammarelay_num_init_outputs(state))
			if numOutputs != lastNumOutputs {
				fmt.Println("calling setColor", numOutputs, lastNumOutputs)
				if err := setColor(lastColor); err != nil {
					log.Println("failed to set color")
				}

				C.wl_display_dispatch_pending(state.display)
				C.wl_display_flush(state.display)

				lastNumOutputs = numOutputs
			}

			log.Printf("Number of outputs: %d\n", numOutputs)

			select {
			case pollNow <- struct{}{}:
			default: // Channel already full, already waiting to poll.
			}

			select {
			case _, ok := <-pollResult:
				if !ok {
					log.Println("pollResult chan closed")
					return
				}

				ret := handlePoll()

				log.Println("handlePoll result", ret)
			case req := <-d.setColorCh:
				handleSetColor(req)
			case <-d.teardownCh:
				C.wl_gammarelay_destroy(d.state)
				return
			}
		}
	}()

	return d, nil
}

type ColorParams struct {
	// Temperature is the color temperature to set. The neutral temperature
	// is 6500
	Temperature int
	// Brightness is the display brightness. The range is [0, 1.0].
	Brightness float32
}

func (p ColorParams) Validate() error {
	if p.Temperature < 1000 || p.Temperature > 10000 {
		return fmt.Errorf("temperature must be in range [1000, 10000], but was %d", p.Temperature)
	}

	if p.Brightness < 0 || p.Brightness > 1 {
		return fmt.Errorf("brightness must be in range [0.0, 1.0], but was: %f", p.Brightness)
	}

	return nil
}

// SetColorTemperature sets the display color and brightness.
func (d *Display) SetColor(ctx context.Context, p ColorParams) error {
	if err := p.Validate(); err != nil {
		return fmt.Errorf("bad params to SetColor: %w", err)
	}

	errCh := make(chan error, 1)

	req := setColorRequest{
		params: p,
		errCh:  errCh,
	}

	select {
	case d.setColorCh <- req:
	case <-ctx.Done():
		return fmt.Errorf("context done set color request: %w", ctx.Err())
	}

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("failed to set color: %w", err)
		}
	case <-ctx.Done():
		return fmt.Errorf("context done set color response: %w", ctx.Err())
	}

	return nil
}

func (d *Display) Close() {
	select {
	case d.teardownCh <- struct{}{}:
	default:
	}
}
