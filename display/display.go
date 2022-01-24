package display

// TODO figure out how to compile C files from different subfolders.

/*
#cgo CFLAGS: -I${SRCDIR}/../protocol
#cgo LDFLAGS: -lwayland-client -lm
#include "wl-gammarelay.h"
#include <wayland-client.h>
#include "../protocol/wlr-gamma-control-unstable-v1-protocol.c"
*/
import "C"
import (
	"fmt"
)

// Display is a wrapper around a Wayland display.
type Display struct {
	state *C.wl_gammarelay_state_t
}

// NewDisplay connects to Wayland server and gets a hold of the display.
//
// TODO The current naive implementation does not have a way to free any of the
// resources acquired.
func New() (*Display, error) {
	state := &C.wl_gammarelay_state_t{}

	ret := C.wl_gammarelay_init(state)
	if ret != 0 {
		panic("failed to initialize gammarelay")
	}

	if state.display == nil {
		return nil, fmt.Errorf("got a nil display")
	}

	// go func() {
	// 	C.wl_display_dispatch(state.display)
	// }()

	return &Display{
		state: state,
	}, nil
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
func (d *Display) SetColor(p ColorParams) error {
	if err := p.Validate(); err != nil {
		return fmt.Errorf("bad params to SetColor: %w", err)
	}

	gamma := [3]C.float{
		C.float(1),
		C.float(1),
		C.float(1),
	}

	ret := C.wl_gammarelay_color_set(d.state, C.color_setting_t{
		temperature: C.int(p.Temperature),
		gamma:       gamma,
		brightness:  C.float(p.Brightness),
	})
	if ret != 0 {
		return fmt.Errorf("failed to set temperature: %d", ret)
	}

	return nil
}
