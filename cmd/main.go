package main

import (
	"fmt"

	"gofr.dev/internal/handler"
	"gofr.dev/internal/service"
	"gofr.dev/internal/store"
	"gofr.dev/pkg/gofr"
)

func main() {
	fmt.Println("🚀 Starting GoFr application...")

	app := gofr.New()

	storeLayer := store.New()
	serviceLayer := service.New(storeLayer)
	handlerLayer := handler.New(serviceLayer)

	// ✅ Register routes (NO DUPLICATE "/")
	app.GET("/", handlerLayer.Home)
	app.POST("/upload", handlerLayer.Upload)
	app.GET("/download", handlerLayer.Download)
	app.POST("/delete", handlerLayer.Delete)

	fmt.Println("✅ Server running on http://localhost:8090")
	app.Start()
}
