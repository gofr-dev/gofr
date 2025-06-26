package main

import (
	"gofr.dev/examples/grpc/grpc-streaming-server/server"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	server.RegisterChatServiceServerWithGofr(app, server.NewChatServiceGoFrServer())

	app.Run()
}
