package types

type Request struct {
	Color *Color `json:"color,omitempty"`
}

type Response struct {
	Error string `json:"message,omitempty"`
	Color *Color `json:"color,omitempty"`
}

type Color struct {
	Temperature string `json:"temperature,omitempty"`
	Brightness  string `json:"brightness,omitempty"`
}
