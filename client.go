package herald

import (
	"encoding/json"

	"github.com/gorilla/websocket"
)

// Client maintains information about an active client.
type Client struct {
	Data            interface{}
	conn            *websocket.Conn
	readChan        chan *Message
	writeChan       chan *Message
	writeClosedChan chan struct{}
	closedChan      chan struct{}
}

func (c *Client) readLoop() {
	defer close(c.closedChan)
	defer func() {
		<-c.writeClosedChan
	}()
	defer close(c.readChan)
	for {
		messageType, p, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}
		m := &Message{}
		if err := json.Unmarshal(p, m); err != nil {
			continue
		}
		c.readChan <- m
	}
}

func (c *Client) writeLoop() {
	defer close(c.writeClosedChan)
	for m := range c.writeChan {
		b, err := json.Marshal(m)
		if err != nil {
			break
		}
		c.conn.WriteMessage(websocket.TextMessage, b)
	}
}

// Close disconnects the client. To ensure the client has completely shut down,
// use the Wait() method.
func (c *Client) Close() {
	c.conn.Close()
}

// Wait waits for the client goroutines to shut down.
func (c *Client) Wait() {
	<-c.closedChan
}
