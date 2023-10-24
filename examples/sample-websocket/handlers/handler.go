package handlers

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/template"
)

func WSHandler(ctx *gofr.Context) (interface{}, error) {
	var (
		mt      int
		message []byte
		err     error
	)

	if ctx.WebSocketConnection != nil {
		for {
			mt, message, err = ctx.WebSocketConnection.ReadMessage()
			if err != nil {
				ctx.Logger.Error("read:", err)
				break
			}

			ctx.Logger.Logf("recv: %v", string(message))

			err = ctx.WebSocketConnection.WriteMessage(mt, message)
			if err != nil {
				ctx.Logger.Error("write:", err)
				break
			}
		}
	}

	return nil, err
}

func HomeHandler(_ *gofr.Context) (interface{}, error) {
	return template.Template{File: "home.html", Type: template.HTML}, nil
}
