//go:build !ignore
// +build !ignore

package nats

//go:generate mockgen -destination=mock_custom_interfaces.go -package=nats -source=./interfaces.go Client,Connection,Subscription,JetStreamContext,Msg
//go:generate mockgen -destination=mock_metrics.go -package=nats -source=./metrics.go
