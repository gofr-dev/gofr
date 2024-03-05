package google

import (
	"context"
	"errors"
	"time"

	"gofr.dev/pkg/gofr/datasource"
	"google.golang.org/api/iterator"
)

func (g *googleClient) Health() (health datasource.Health) {
	const contextTimeoutDuration = 50

	health.Details = make(map[string]interface{})

	health.Status = datasource.StatusUp
	health.Details["backend"] = "GOOGLE"

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeoutDuration*time.Millisecond)
	defer cancel()

	it := g.client.Topics(ctx)

	topics := make(map[string]interface{})

	for {
		topic, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}

		if err != nil {
			health.Status = datasource.StatusDown

			break
		}

		if topic != nil {
			topics[topic.ID()] = topic
		}
	}

	health.Details["writers"] = topics

	ctx, cancel = context.WithTimeout(context.Background(), contextTimeoutDuration*time.Millisecond)
	defer cancel()

	subIt := g.client.Subscriptions(ctx)

	subscriptions := make(map[string]interface{})

	for {
		subcription, err := subIt.Next()
		if errors.Is(err, iterator.Done) {
			break
		}

		if err != nil {
			health.Status = datasource.StatusDown

			break
		}

		if subcription != nil {
			subscriptions[subcription.ID()] = subcription
		}
	}

	health.Details["readers"] = subscriptions

	return health
}
