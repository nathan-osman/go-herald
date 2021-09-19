## go-herald

[![Build Status](https://travis-ci.com/nathan-osman/go-herald.svg?branch=main)](https://travis-ci.com/nathan-osman/go-herald)
[![Coverage Status](https://coveralls.io/repos/github/nathan-osman/go-herald/badge.svg?branch=main)](https://coveralls.io/github/nathan-osman/go-herald?branch=main)
[![Go Report Card](https://goreportcard.com/badge/github.com/nathan-osman/go-herald)](https://goreportcard.com/report/github.com/nathan-osman/go-herald)
[![Go Reference](https://pkg.go.dev/badge/github.com/nathan-osman/go-herald.svg)](https://pkg.go.dev/github.com/nathan-osman/go-herald)
[![MIT License](https://img.shields.io/badge/license-MIT-9370d8.svg?style=flat)](https://opensource.org/licenses/MIT)

go-herald provides a very simple way for clients to exchange messages with a central server using WebSockets.

### Basic Usage

Begin by importing the package with:

```golang
import "github.com/nathan-osman/go-herald"
```

Next, create an instance of the `Herald` class with:

```golang
herald := herald.New(&herald.Config{})
```

When a WebSocket connection is received, call the `AddClient` method:

```golang
func someHandler(w http.ResponseWriter, r *http.Request) {
    herald.AddClient(w, r, nil)
}
```

The third parameter is an `interface{}` that can be used to associate custom data with that particular client.

To send messages to the clients, prepare them with `NewMessage` function and use the `Herald`'s `Send` method:

```golang
msg, err := herald.NewMessage("test", "data")
// TODO: handle err
herald.Send(msg)
```

A JavaScript client for the above example might look something like the following:

```javascript
let ws = new WebSocket("ws://server/ws");

ws.send(
  JSON.stringify(
    { type: "test", data: "data" }
  )
);
```

By default, messages received by the `Herald` are simply rebroadcast to all other connected clients.
