package herald

// Config stores configuration information for creating Herald instances.
type Config struct {

	// ReceiverFunc receives messages as they arrive from clients. If this
	// field is nil, the messages are simply rebroadcast to all clients.
	ReceiverFunc func(m *Message)

	// ClientAddedFunc is invoked when a new client connects.
	ClientAddedFunc func(c *Client)

	// ClientRemovedFunc is invoked when a client disconnects.
	ClientRemovedFunc func(c *Client)
}
