package main

// TODO figure out how to compile C files from different subfolders.

/*
#cgo LDFLAGS: -lwayland-client -lm
#include "wl-gammarelay.h"
*/
import "C"
import (
	"fmt"
	"time"
)

// Display is a wrapper around a Wayland display.
type Display struct {
	display *C.struct_wl_display
}

// NewDisplay connects to Wayland server and gets a hold of the display.
//
// TODO The current naive implementation does not have a way to free any of the
// resources acquired.
func NewDisplay() (*Display, error) {
	var disp *C.struct_wl_display

	ret := C.wl_gammarelay_init(&disp)
	if ret != 0 {
		panic("failed to initialize gammarelay")
	}

	if disp == nil {
		return nil, fmt.Errorf("got a nil display")
	}

	return &Display{
		display: disp,
	}, nil
}

// SetColorParams are parameters for setting color temperature.
type SetColorParams struct {
	// Temperature is the color temperature to set. The neutral temperature
	// is 6500
	Temperature int
	// Brightness is the display brightness. The range is [0, 1.0].
	Brightness float32
}

// SetColorTemperature sets the display color and brightness.
func (d *Display) SetColor(setting SetColorParams) error {
	gamma := [3]C.float{
		C.float(1),
		C.float(1),
		C.float(1),
	}

	ret := C.wl_gammarelay_color_set(d.display, C.color_setting_t{
		temperature: C.int(setting.Temperature),
		gamma:       gamma,
		brightness:  C.float(setting.Brightness),
	})
	if ret != 0 {
		return fmt.Errorf("failed to set temperature: %d", ret)
	}

	return nil
}

// main is a test function for proof-of-concept.
func main() {
	d, err := NewDisplay()
	if err != nil {
		panic(err)
	}

	params := SetColorParams{
		Temperature: 3000,
		Brightness:  1.0,
	}

	if err := d.SetColor(params); err != nil {
		panic(err)
	}

	time.Sleep(5 * time.Second)
}
