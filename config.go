package herald

// ReceiverFunc is invoked when messages are received from clients.
type ReceiverFunc func(m *Message)

// Config stores configuration information for creating Herald instances.
type Config struct {

	// ReceiverFunc receives messages as they arrive from clients. If this
	// field is nil, the messages are simply rebroadcast to all clients.
	ReceiverFunc ReceiverFunc
}
