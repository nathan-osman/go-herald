package herald

import (
	"net/http"
	"reflect"
	"sync"

	"github.com/gorilla/websocket"
)

type sendParams struct {
	message *Message
	clients []*Client
}

// Herald maintains a set of WebSocket connections and facilitates the exchange
// of messages between them.
type Herald struct {

	// MessageHandler processes messages as they are coming in. If nil,
	// messages will simply be re-broadcast to all clients.
	MessageHandler func(message *Message, client *Client)

	// ClientAddedHandler processes new clients after they connect. This field
	// is optional.
	ClientAddedHandler func(client *Client)

	// ClientRemovedHandler processes clients after they disconnect. This field
	// is optional.
	ClientRemovedHandler func(client *Client)

	mutex          sync.RWMutex
	upgrader       *websocket.Upgrader
	clients        []*Client
	addClientChan  chan *Client
	sendParamsChan chan *sendParams
	closeChan      chan struct{}
	closedChan     chan struct{}
}

func (h *Herald) run() {
	defer close(h.closedChan)
	shuttingDown := false
	for {

		// The list of select cases needs to assembled at runtime so that the
		// read and closed channels from the clients can be included

		var cases []reflect.SelectCase
		for _, c := range h.clients {
			cases = append(
				cases,
				reflect.SelectCase{
					Dir:  reflect.SelectRecv,
					Chan: reflect.ValueOf(c.readChan),
				},
				reflect.SelectCase{
					Dir:  reflect.SelectRecv,
					Chan: reflect.ValueOf(c.closedChan),
				},
			)
		}

		var (

			// Remember the number of cases for the client channels
			numClientCases = len(cases)

			// Utility function for adding a new case and receiving its index
			addCase = func(v reflect.Value) (i int) {
				i = len(cases)
				cases = append(cases, reflect.SelectCase{
					Dir:  reflect.SelectRecv,
					Chan: v,
				})
				return
			}

			// Add cases for the addClient and sendParams channel
			addClientIdx  = addCase(reflect.ValueOf(h.addClientChan))
			sendParamsIdx = addCase(reflect.ValueOf(h.sendParamsChan))
			closeIdx      = -1
		)

		// Add a case for the close channel if not shutting down
		if !shuttingDown {
			closeIdx = addCase(reflect.ValueOf(h.closeChan))
		}

		// Perform the select
		chosen, recv, recvOK := reflect.Select(cases)
		switch {

		// Message received from a client or client disconnected
		case chosen < numClientCases:
			var (
				clientIdx = chosen / 2
				c         = h.clients[clientIdx]
			)

			// If the index is divisible by 2, it is the read channel;
			// otherwise it is the closed channel
			if chosen%2 == 0 {
				if recvOK {

					// A value was received; handle it
					m := recv.Interface().(*Message)
					h.MessageHandler(m, c)
				} else {

					// If the read channel is closed, nothing more can be read;
					// set the channel to nil to prevent short-circuiting the
					// select{} statement; close the write channel and set it
					// to nil since writing to the socket is impossible
					close(c.writeChan)
					c.readChan = nil
					c.writeChan = nil
				}
			} else {

				// Remove the client from the list since it has completely shut
				// down by this point; if the loop is being shut down and this
				// was the last client, terminate the loop
				func() {
					h.mutex.Lock()
					defer h.mutex.Unlock()
					h.clients = append(h.clients[:clientIdx], h.clients[clientIdx+1:]...)
				}()
				if h.ClientRemovedHandler != nil {
					h.ClientRemovedHandler(c)
				}
				if shuttingDown && len(h.clients) == 0 {
					return
				}
			}

		// New client has connected
		case chosen == addClientIdx:
			c := recv.Interface().(*Client)
			if h.ClientAddedHandler != nil {
				h.ClientAddedHandler(c)
			}
			func() {
				h.mutex.Lock()
				defer h.mutex.Unlock()
				h.clients = append(h.clients, c)
			}()

		// Message to send
		case chosen == sendParamsIdx:
			p := recv.Interface().(*sendParams)
			if p.clients == nil {
				p.clients = h.clients
			}
			for _, c := range p.clients {
				if c.writeChan != nil {
					select {
					case c.writeChan <- p.message:
					default:
						c.conn.Close()
					}
				}
			}

		// Start shutting all of the clients down and return when complete
		case chosen == closeIdx:
			if len(h.clients) > 0 {
				for _, c := range h.clients {
					c.conn.Close()
				}
				shuttingDown = true
			} else {
				return
			}
		}
	}
}

// New creates and begins initializing a new Herald instance. The Herald is not
// started until the Start() method is invoked.
func New() *Herald {
	h := &Herald{
		upgrader:       &websocket.Upgrader{},
		addClientChan:  make(chan *Client),
		sendParamsChan: make(chan *sendParams),
		closeChan:      make(chan struct{}),
		closedChan:     make(chan struct{}),
	}
	h.MessageHandler = func(m *Message, c *Client) {
		h.Send(m, nil)
	}
	return h
}

// Start completes initialization and begins processing messages.
func (h *Herald) Start() {
	go h.run()
}

// AddClient adds a new WebSocket client and begins exchanging messages.
func (h *Herald) AddClient(w http.ResponseWriter, r *http.Request, data interface{}) (*Client, error) {
	c, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	client := &Client{
		Data:            data,
		conn:            c,
		readChan:        make(chan *Message),
		writeChan:       make(chan *Message, 10),
		writeClosedChan: make(chan struct{}),
		closedChan:      make(chan struct{}),
	}
	go client.readLoop()
	go client.writeLoop()
	h.addClientChan <- client
	return client, nil
}

// Send sends the specified message to the client specified in the message or
// all clients if nil. The send operation takes place in a separate goroutine
// to enable the call to be made from handlers without triggering a deadlock.
func (h *Herald) Send(message *Message, clients []*Client) {
	go func() {
		h.sendParamsChan <- &sendParams{
			message: message,
			clients: clients,
		}
	}()
}

// Clients returns a slice of all currently connected clients.
func (h *Herald) Clients() []*Client {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return h.clients
}

// SetCheckOrigin provides a function that will be invoked for every new
// connection. If the function returns true, it will be allowed.
func (h *Herald) SetCheckOrigin(fn func(*http.Request) bool) {
	h.upgrader.CheckOrigin = fn
}

// Close disconnects all clients and stops exchanging messages.
func (h *Herald) Close() {
	close(h.closeChan)
	<-h.closedChan
}
