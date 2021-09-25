package herald

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

const (
	messageType = "test"
	clientData  = "test"
)

func TestHerald(t *testing.T) {

	// Create a message for sending
	message, err := NewMessage(messageType, nil)
	if err != nil {
		t.Fatal(err)
	}

	var (
		receivedChan      = make(chan bool)
		clientAddedChan   = make(chan bool)
		clientRemovedChan = make(chan bool)

		h = New()
	)

	// Confirm that the correct message is received
	h.MessageHandler = func(m *Message) {
		if !reflect.DeepEqual(message, m) {
			t.Fatal("message does not match")
		}
		close(receivedChan)
	}

	// Confirm that the correct client data is present
	h.ClientAddedHandler = func(c *Client) {
		if !reflect.DeepEqual(c.Data, clientData) {
			t.Fatal("client data does not match")
		}
		close(clientAddedChan)
	}

	// Confirm that the correct client data is present
	h.ClientRemovedHandler = func(c *Client) {
		if !reflect.DeepEqual(c.Data, clientData) {
			t.Fatal("client data does not match")
		}
		close(clientRemovedChan)
	}

	// Start the herald
	h.Start()
	defer h.Close()

	// Create the server, upgrading connections as they come in
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.AddClient(w, r, clientData)
	}))
	defer s.Close()

	// Change the protocol to ws
	wsURL := strings.Replace(s.URL, "http", "ws", 1)

	// Create the client connection
	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	// Wait for the client connection
	<-clientAddedChan

	// Confirm the client is in the list
	if len(h.Clients()) != 1 {
		t.Fatal("Clients() should return 1")
	}

	// Have the client send a message
	b, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.WriteMessage(websocket.TextMessage, b); err != nil {
		t.Fatal(err)
	}

	// Wait for message
	<-receivedChan

	// Disconnect the client
	c.Close()

	// Wait for client disconnect
	<-clientRemovedChan
}
