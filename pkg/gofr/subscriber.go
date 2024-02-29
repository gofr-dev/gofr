package gofr

import (
	"sync"

	"gofr.dev/pkg/gofr/container"
)

type SubscribeFunc func(c *Context) error

type SubscriptionManager struct {
	*container.Container
	subscriptions map[string]SubscribeFunc
	wg            sync.WaitGroup // WaitGroup to wait for subscriber goroutines to finish
}

func newSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		subscriptions: make(map[string]SubscribeFunc),
	}
}
