package herald

// Config stores configuration information for creating Herald instances.
type Config struct {

	// ReceiverFunc receives messages as they arrive from clients. If this
	// field is nil, the messages are simply rebroadcast to all clients.
	ReceiverFunc func(m *Message)

	// ClientAddedFunc is invoked when a new client connects. The clients slice
	// contains a list of all existing clients, not including c.
	ClientAddedFunc func(clients []*Client, c *Client)

	// ClientRemovedFunc is invoked when a client disconnects. The clients
	// slice contains a list of all other clients, not including c.
	ClientRemovedFunc func(clients []*Client, c *Client)
}
