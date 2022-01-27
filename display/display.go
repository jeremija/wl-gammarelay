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
	"sync"

	"github.com/peer-calls/log"
)

// Display is a wrapper around a Wayland display.
type Display struct {
	state *C.wl_gammarelay_t

	log log.Logger
	wg  sync.WaitGroup

	setColorCh chan setColorRequest
	teardownCh chan struct{}
}

type setColorRequest struct {
	params ColorParams
	errCh  chan<- error
}

// New connects to Wayland server and gets a hold of the display.
func New(logger log.Logger) (*Display, error) {
	state := C.wl_gammarelay_init()
	if state == nil {
		return nil, fmt.Errorf("failed to connect to display")
	}

	if state.display == nil {
		return nil, fmt.Errorf("got a nil display")
	}

	pollNow := make(chan struct{}, 1)
	pollResult := make(chan struct{}, 1)

	d := &Display{
		state:      state,
		log:        logger,
		setColorCh: make(chan setColorRequest),
		teardownCh: make(chan struct{}, 1),
	}

	doneCh := make(chan struct{})

	d.wg.Add(2)

	go func() {
		defer d.wg.Done()
		defer d.log.Trace("poll goroutine done", nil)

		for {
			select {
			case <-pollNow:
				d.log.Trace("wl_gammarelay_poll CALL", nil)
				ret := C.wl_gammarelay_poll(state)
				d.log.Trace("wl_gammarelay_poll DONE", nil)

				if ret < 0 {
					d.log.Trace("wl_gammarelay_poll pollResult close", log.Ctx{
						"ret": ret,
					})
					close(pollResult)
					return
				}

				if ret == 0 {
					// timeout
					continue
				}

				select {
				case pollResult <- struct{}{}:
				case <-doneCh:
					return
				}
			case <-doneCh:
				return
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
		d.log.Trace("wl_display_dispatch CALL", nil)
		ret := int(C.wl_display_dispatch(state.display))
		d.log.Trace("wl_display_dispatch DONE", nil)
		return ret
	}

	pollNow <- struct{}{}

	go func() {
		defer d.wg.Done()
		defer close(doneCh)

		for {
			C.wl_display_dispatch_pending(state.display)
			C.wl_display_flush(state.display)

			numOutputs := int(C.wl_gammarelay_num_init_outputs(state))
			if numOutputs != lastNumOutputs {
				d.log.Trace("setColor CALL", log.Ctx{
					"num_outputs":      numOutputs,
					"last_num_ouptuts": lastNumOutputs,
				})

				if err := setColor(lastColor); err != nil {
					d.log.Error("Failed to set color", err, nil)
				}

				C.wl_display_dispatch_pending(state.display)
				C.wl_display_flush(state.display)

				lastNumOutputs = numOutputs
			}

			d.log.Trace("Main loop iteration", log.Ctx{
				"num_outputs": numOutputs,
			})

			select {
			case _, ok := <-pollResult:
				if !ok {
					d.log.Trace("Main loop pollResult chan closed", nil)
					return
				}

				d.log.Trace("handlePoll CALL", nil)
				_ = handlePoll()
				d.log.Trace("handlePoll DONE", nil)

				d.log.Trace("pollNow SEND", nil)
				pollNow <- struct{}{}
				d.log.Trace("pollNow SENT", nil)
			case req := <-d.setColorCh:
				handleSetColor(req)
			case <-d.teardownCh:
				C.wl_gammarelay_interrupt(d.state)
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
	if d.state == nil {
		return
	}

	select {
	case d.teardownCh <- struct{}{}:
		d.wg.Wait()

		// Only destroy after all goroutines have finished.
		C.wl_gammarelay_destroy(d.state)

		d.state = nil
	default:
	}
}
