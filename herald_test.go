package herald

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const (
	receiveTimeout = 5 * time.Second

	messageType1 = "test1"
	messageType2 = "test2"

	clientData = "test"
)

func newTestMessage(t *testing.T, messageType string) *Message {
	m, err := NewMessage(messageType, nil)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

type testServer struct {
	herald          *Herald
	receivedWG      *sync.WaitGroup
	clientAddedWG   *sync.WaitGroup
	clientRemovedWG *sync.WaitGroup
}

func newTestServer() *testServer {
	s := &testServer{
		herald:          New(),
		receivedWG:      &sync.WaitGroup{},
		clientAddedWG:   &sync.WaitGroup{},
		clientRemovedWG: &sync.WaitGroup{},
	}
	s.herald.MessageHandler = func(m *Message, c *Client) {
		s.receivedWG.Done()
	}
	s.herald.ClientAddedHandler = func(c *Client) {
		s.clientAddedWG.Done()
	}
	s.herald.ClientRemovedHandler = func(c *Client) {
		s.clientRemovedWG.Done()
	}
	s.herald.Start()
	return s
}

type testClient struct {
	client *Client
	conn   *websocket.Conn
}

func newTestClient(t *testing.T, s *testServer) *testClient {
	s.clientAddedWG.Add(1)
	var (
		c      = &testClient{}
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			client, err := s.herald.AddClient(w, r, clientData)
			if err != nil {
				t.Log(err)
				t.Fail()
				return
			}
			c.client = client
		}))
		addr         = strings.Replace(server.URL, "http", "ws", 1)
		conn, _, err = websocket.DefaultDialer.Dial(addr, nil)
	)
	if err != nil {
		t.Fatal(err)
	}
	c.conn = conn
	server.Close()
	s.clientAddedWG.Wait()
	return c
}

func (c *testClient) send(t *testing.T, s *testServer, m *Message) {
	s.receivedWG.Add(1)
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, b); err != nil {
		t.Fatal(err)
	}
	s.receivedWG.Wait()
}

func (c *testClient) receive(t *testing.T, s *testServer, m *Message) {
	msgChan := make(chan []byte)
	go func() {
		defer close(msgChan)
		_, p, err := c.conn.ReadMessage()
		if err != nil {
			t.Log(err)
			t.Fail()
			return
		}
		msgChan <- p
	}()
	select {
	case p, ok := <-msgChan:
		if !ok {
			return
		}
		receivedMessage := &Message{}
		if err := json.Unmarshal(p, receivedMessage); err != nil {
			t.Fatal(err)
		}
		if receivedMessage.Type != m.Type {
			t.Fatalf("%s != %s", receivedMessage.Type, m.Type)
		}
	case <-time.After(receiveTimeout):
		t.Fatal("timeout reached")
	}
}

func (c *testClient) close(s *testServer) {
	s.clientRemovedWG.Add(1)
	c.conn.Close()
	s.clientRemovedWG.Wait()
}

func (c *testClient) verifyDisconnected(t *testing.T) {
	b := make([]byte, 32)
	if _, err := c.conn.UnderlyingConn().Read(b); !errors.Is(err, io.EOF) {
		t.Fatal("client was not disconnected")
	}
}

func TestHeraldConnect(t *testing.T) {

	// Create the server
	s := newTestServer()
	defer s.herald.Close()

	// Create a client, verify that it is connected, and close it
	c := newTestClient(t, s)
	if !reflect.DeepEqual(s.herald.Clients(), []*Client{c.client}) {
		t.Fatal("client list does not match")
	}
	c.close(s)
}

func TestHeraldReceive(t *testing.T) {

	// Create the server
	s := newTestServer()
	defer s.herald.Close()

	// Create a client, send a message, and close it
	c := newTestClient(t, s)
	c.send(t, s, newTestMessage(t, messageType1))
	c.close(s)
}

func TestHeraldSend(t *testing.T) {

	// Create the server
	s := newTestServer()
	defer s.herald.Close()

	// Create two clients and two unique messages
	var (
		c1 = newTestClient(t, s)
		c2 = newTestClient(t, s)

		m1 = newTestMessage(t, messageType1)
		m2 = newTestMessage(t, messageType2)
	)

	// Send a separate message to each client
	s.herald.Send(m1, []*Client{c1.client})
	s.herald.Send(m2, []*Client{c2.client})

	// Receive the messages
	c1.receive(t, s, m1)
	c2.receive(t, s, m2)

	// Send a message to all clients
	s.herald.Send(m1, nil)

	// Receiv the messages
	c1.receive(t, s, m1)
	c2.receive(t, s, m1)

	// Close the client
	c1.close(s)
	c2.close(s)
}

func TestHeraldClose(t *testing.T) {

	// Create the server and a client
	var (
		s = newTestServer()
		c = newTestClient(t, s)
	)

	// Disconnect the server
	s.clientRemovedWG.Add(1)
	s.herald.Close()

	// Ensure the client was disconnected
	c.verifyDisconnected(t)
}

func TestClientClose(t *testing.T) {

	// Create the server and a client
	var (
		s = newTestServer()
		c = newTestClient(t, s)
	)

	// Close the client from the server's side and wait
	s.clientRemovedWG.Add(1)
	c.client.Close()
	c.client.Wait()

	// Ensure the client was disconnected
	c.verifyDisconnected(t)
}
