package types

// Request is a message sent from the client to the daemon. In the future we
// might add more fields.
type Request struct {
	Color *Color `json:"color,omitempty"`
}

// Response is a message sent from the daemon to the client in response to a
// Request.
type Response struct {
	// Error will be set to a non-empty string when the operation was
	// unsuccessful.
	Error string `json:"message,omitempty"`
	// Color will contain the current absolute temperature settings.
	Color *Color `json:"color,omitempty"`
}

// Color defines the color temperature settings.
type Color struct {
	// Temperature is a relative or absolute integer. Strings containing a + or -
	// prefix will be treated as relative.
	Temperature string `json:"temperature,omitempty"`
	// Temperature is a relative or absolute float value. Strings containing a +
	// or - prefix will be treated as relative.
	Brightness string `json:"brightness,omitempty"`
}
