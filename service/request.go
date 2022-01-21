package service

import "github.com/jeremija/wl-gammarelay/display"

type Request struct {
	ColorParams *ColorParams `json:"colorParams,omitempty"`
}

type ColorParams struct {
	display.ColorParams
	TemperatureIsRealtive bool
	BrightnessIsRelative  bool
}

func (s *ColorParams) AbsoluteColorParams(prev display.ColorParams) display.ColorParams {
	p := s.ColorParams

	if s.TemperatureIsRealtive {
		p.Temperature = prev.Temperature + p.Temperature
	}

	if s.BrightnessIsRelative {
		p.Brightness = prev.Brightness + p.Brightness
	}

	return p
}
