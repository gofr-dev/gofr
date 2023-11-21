# WebSocket

WebSockets have enhanced HTTP connections that remain active until either the client or the server terminates the link. We can do duplex websocket communication using GoFr i.e we can communicate to and from the server from our client using a single connection.

In order to communicate using web socket protocol with the client, a web socket connection is required.

Gofr initialises a web socket connection in the gofr context object. A web socket connection is created when http request contains the following headers and the value:-

| Header                | Value                |
| --------------------- | -------------------- |
| Connection            | upgrade              |
| Upgrade               | websocket            |
| Sec-Websocket-Version | <Websocket version\> |
| Sec-WebSocket-Key     | <Websocket key\>     |

`Sec-Websocket-Key` is meant to prevent proxies from caching the request, by sending a random key. It does not provide any authentication.

## Usage

```go
package main

import (
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

func main() {
	// initialise gofr object
	app := gofr.New()

	app.GET("/websocket", func(c *gofr.Context) (interface{}, error) {
		// c.WebSocketConnection is the websocket connection created. It is created only if request headers contain all the 4 header-value pair mentioned above.
		if c.WebSocketConnection != nil {
			for {
				// to read the data invoke ReadMessage method
				msgType, data, err := c.WebSocketConnection.ReadMessage()
				if err != nil {
					return nil, err
				}

				// log the data
				c.Logger.Info("Message received: ", string(data))

				// to send the data invoke WriteMessage method
				err = c.WebSocketConnection.WriteMessage(msgType, []byte("send message using web socket connection"))
				if err != nil {
					return nil, err
				}
			}
		}

		return nil, errors.MissingParam{Param: checkWebSocketHeaders(c)}
	})

	// Starts the server, it will listen on the default port 8000.
	// it can be over-ridden through configs
	app.Start()
}

func checkWebSocketHeaders(c *gofr.Context) []string {
	missingHeaders := []string{}
	requiredHeaders := []string{"Connection", "Upgrade", "Sec-Websocket-Version", "Sec-WebSocket-Key"}

	for _, key := range requiredHeaders {
		if c.Header(key) == "" {
			missingHeaders = append(missingHeaders, key)
		}
	}

	return missingHeaders
}
```
