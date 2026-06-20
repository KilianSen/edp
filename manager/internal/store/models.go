package store

import (
	"encoding/json"
	"time"
)

// Instance is a registered edp instance the manager orchestrates.
type Instance struct {
	ID        int64     `json:"id"`
	Label     string    `json:"label"`
	BaseURL   string    `json:"base_url"`
	APIToken  string    `json:"api_token,omitempty"` // encrypted at rest; never returned by the API
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MarshalJSON blanks the API token so it never leaves via the API (the token is
// still accepted on input via the default unmarshal, and is available to the
// fan-out client which reads the struct field directly).
func (i Instance) MarshalJSON() ([]byte, error) {
	type alias Instance
	c := alias(i)
	c.APIToken = ""
	return json.Marshal(c)
}
