package main

import (
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

	gogprc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcServer "gofr.dev/examples/sample-grpc/handler/grpc"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/service"
)

func main() {
	app := gofr.New()

	// Bypass header validation during API calls
	app.Server.ValidateHeaders = false

	conn, err := gogprc.Dial(app.Config.Get("GRPC_SERVER"), gogprc.WithTransportCredentials(insecure.NewCredentials()),
		gogprc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		gogprc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()))
	if err != nil {
		app.Logger.Error(err)
	}

	defer conn.Close()

	c := grpcServer.NewExampleServiceClient(conn)

	url := app.Config.Get("SAMPLE_API_URL")

	app.GET("/trace", func(ctx *gofr.Context) (interface{}, error) {
		span := ctx.Trace("some-sample-work")
		<-time.After(time.Millisecond * 1) // Waiting for 1ms to simulate workload
		span.End()

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			svc := service.NewHTTPServiceWithOptions(url, ctx.Logger, nil)
			_, _ = svc.Get(ctx, "hello", nil)
			wg.Done()
		}()

		// Ping redis 2 times concurrently and wait.
		count := 2
		wg.Add(count)
		for i := 0; i < count; i++ {
			go func() {
				ctx.Redis.Ping(ctx)
				wg.Done()
			}()
		}
		wg.Wait()

		// grpc call
		_, err := c.Get(ctx.Context, &grpcServer.ID{Id: "1"})
		if err != nil {
			return nil, err
		}

		return "ok", nil
	})

	app.Start()
}
