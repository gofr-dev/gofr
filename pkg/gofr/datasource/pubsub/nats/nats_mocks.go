package nats

//go:generate mockgen -destination=mock_client.go -package=nats -source=./interfaces.go Client,Subscription
//go:generate mockgen -destination=mock_jetstream.go -package=nats github.com/nats-io/nats.go/jetstream JetStream,Consumer,Msg
//go:generate mockgen -destination=mock_metrics.go -package=nats -source=./metrics.go
