package main

import (
	"fmt"
	"gofr.dev/examples/grpc-server/grpc"
	"gofr.dev/pkg/gofr"
)

func main() {
	parseInterface, err := ParseInterface("interface.go")
	if err != nil {
		return
	}

	fmt.Println(parseInterface)

	clientCode, serverCode, err := GenerateCode(parseInterface)
	if err != nil {
		return
	}

	fmt.Println(clientCode)

	fmt.Print(serverCode)

	app := gofr.New()

	grpc.RegisterHelloServer(app, grpc.Server{})

	app.Run()
}
