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
		// read channels from the currently active clients can be included in
		// the loop

		var cases []reflect.SelectCase
		for _, c := range h.clients {
			cases = append(cases, reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(c.readChan),
			})
		}

		// Add the channel for adding new clients
		var (
			addCase = func(v reflect.Value) (i int) {
				i = len(cases)
				cases = append(cases, reflect.SelectCase{
					Dir:  reflect.SelectRecv,
					Chan: v,
				})
				return
			}
			addClientIdx  = addCase(reflect.ValueOf(h.addClientChan))
			sendParamsIdx = addCase(reflect.ValueOf(h.sendParamsChan))
			closeIdx      = -1
		)

		// Add the channel for closing if not shutting down
		if !shuttingDown {
			closeIdx = addCase(reflect.ValueOf(h.closeChan))
		}

		// Perform the select
		chosen, recv, recvOK := reflect.Select(cases)
		switch {

		// Message received from a client or client disconnected
		case chosen < len(h.clients):
			if recvOK {
				var (
					m = recv.Interface().(*Message)
					c = h.clients[chosen]
				)
				h.MessageHandler(m, c)
			} else {
				c := h.clients[chosen]
				close(c.writeChan)
				func() {
					h.mutex.Lock()
					defer h.mutex.Unlock()
					h.clients = append(h.clients[:chosen], h.clients[chosen+1:]...)
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
				select {
				case c.writeChan <- p.message:
				default:
					c.conn.Close()
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
func (h *Herald) AddClient(w http.ResponseWriter, r *http.Request, data interface{}) error {
	c, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	client := &Client{
		Data:      data,
		conn:      c,
		readChan:  make(chan *Message),
		writeChan: make(chan *Message, 10),
	}
	go client.readLoop()
	go client.writeLoop()
	h.addClientChan <- client
	return nil
}

// Send sends the specified message to the client specified in the message or
// all clients if nil.
func (h *Herald) Send(message *Message, clients []*Client) {
	h.sendParamsChan <- &sendParams{
		message: message,
		clients: clients,
	}
}

// Clients returns a slice of all currently connected clients.
func (h *Herald) Clients() []*Client {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return h.clients
}

// Close disconnects all clients and stops exchanging messages.
func (h *Herald) Close() {
	close(h.closeChan)
	<-h.closedChan
}
