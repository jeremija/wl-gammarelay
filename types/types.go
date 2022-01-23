package types

type SubscriptionKey string

const (
	SubscriptionKeyColor SubscriptionKey = "color"
)

// Request is a message sent from the client to the daemon.
type Request struct {
	// Color to set.
	Color *Color `json:"color,omitempty"`
	// Subscribe to specific subscription keys.
	Subscribe []SubscriptionKey `json:"subscribe,omitempty"`
	// Unsubscribe from specific subscription keys.
	Unsubscribe []SubscriptionKey `json:"unsubscribe,omitempty"`
}

// Update is a subscription update.
type Update struct {
	Key   SubscriptionKey `json:"key"`
	Color *Color          `json:"color,omitempty"`
}

// Response is a message sent from the daemon to the client in response to a
// Request.
type Response struct {
	// Error will be set to a non-empty string when the operation was
	// unsuccessful.
	Error string `json:"message,omitempty"`
	// Color will contain the response to the Color request. It will contain the
	// updated color state.
	Color *Color `json:"color,omitempty"`
	// Update contains any subscription updates.
	Updates []Update `json:"updates,omitempty"`
	// Subscribed confirms the subscribed keys.
	Subscribed []SubscriptionKey `json:"subscribed,omitempty"`
	// Unsubscribed confirms the unsubscribed keys.
	Unsubscribed []SubscriptionKey `json:"unusbscribed,omitempty"`
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
