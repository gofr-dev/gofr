# Websockets

WebSockets provide a full-duplex communication channel over a single, long-lived connection, making them ideal for 
real-time applications like chat, notifications, and live updates. GoFr provides a convenient way to integrate websockets
into your application. By leveraging GoFr's WebSocket support and customizable upgrader options,
you can efficiently manage real-time communication in your applications.

## Usage in GoFr

Here is a simple example to set up a WebSocket server in GoFr:

```go
package main

import (
    "fmt"
    
    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/logging"
    "gofr.dev/pkg/gofr/websocket"
)

func main() {
    app := gofr.New()

    app.GET("/ws", WSHandler)

    app.Run()
}

func WSHandler(c *gofr.Context) (interface{}, error) {
    conn := c.GetWebsocketConnection().Conn
    if conn == nil {
        return nil, fmt.Errorf("websocket connection not found in context")
    }

    handleWebSocketMessages(&websocket.Connection{Conn: conn}, c.Logger)

    return nil, nil
}

func handleWebSocketMessages(conn *websocket.Connection, logger logging.Logger) {
    defer conn.Close()
    for {
        _, msg, err := conn.ReadMessage()
        if err != nil {
            logger.Errorf("Unexpected close error: %v", err)
            break
        }

        logger.Infof("Received message: %s", msg)

        // Echo the message back
        err = conn.WriteMessage(websocket.TextMessage, msg)
        if err != nil {
            logger.Errorf("Error writing message: %v", err)
            break
        }
    }
}
```

## Configuration Options
GoFr allows you to customize the WebSocket upgrader with several options. You can set these options using the 
`websocket.NewWSUpgrader` function. Here are the list of options you can apply to your websocket upgrader using GoFr.

- `HandshakeTimeout (WithHandshakeTimeout)`: Sets the handshake timeout.
- `ReadBufferSize (WithReadBufferSize)`: Sets the size of the read buffer.
- `WriteBufferSize (WithWriteBufferSize)`: Sets the size of the write buffer.
- `Subprotocols (WithSubprotocols)`: Sets the supported sub-protocols.
- `Error (WithError)`:  Sets a custom error handler.
- `CheckOrigin (WithCheckOrigin)`: Sets a custom origin check function.
- `Compression (WithCompression)`:  Enables compression.

# Example:
You can configure the Upgrader by creating a chain of option functions provided by GoFr.

```go
wsUpgrader := websocket.NewWSUpgrader(
  websocket.WithHandshakeTimeout(5 * time.Second), // Set handshake timeout
  websocket.WithReadBufferSize(2048),              // Set read buffer size
  websocket.WithWriteBufferSize(2048),             // Set write buffer size
  websocket.WithSubprotocols("chat", "binary"),    // Specify subprotocols
  websocket.WithCompression(),                     // Enable compression
)
```

## Override Default WebSocket Upgrader

To use the custom upgrader, you need to override the default upgrader in your GoFr application:

``` go
func (a *App) OverrideWebsocketUpgrader(wsUpgrader websocket.Upgrader) {
    a.container.WebSocketUpgrader.Upgrader = wsUpgrader
}
```
This allows you to apply the custom configuration to the WebSocket upgrader used by your application.


