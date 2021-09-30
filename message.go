package herald

import (
	"encoding/json"
)

// Message stores information for broadcasting to other clients. The Client
// field is a pointer to either the client who sent the message or the one that
// should receive it.
type Message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// NewMessage creates a new Message instance of the specified type with the
// specified data.
func NewMessage(messageType string, v interface{}) (*Message, error) {
	m := &Message{
		Type: messageType,
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	m.Data = json.RawMessage(b)
	return m, nil
}
