package google

import (
	"context"
	"errors"
	"time"

	"gofr.dev/pkg/gofr/health"
	"google.golang.org/api/iterator"

	"gofr.dev/pkg/gofr/datasource"
)

func (g *googleClient) Health() (h health.Health) {
	h.Details = make(map[string]interface{})

	var writerStatus, readerStatus string

	h.Status = datasource.StatusUp
	h.Details["projectID"] = g.Config.ProjectID
	h.Details["backend"] = "GOOGLE"

	writerStatus, h.Details["writers"] = g.getWriterDetails()
	readerStatus, h.Details["readers"] = g.getReaderDetails()

	if readerStatus == datasource.StatusDown || writerStatus == datasource.StatusDown {
		h.Status = datasource.StatusDown
	}

	return h
}

//nolint:dupl // getWriterDetails provides the publishing details for current google publishers.
func (g *googleClient) getWriterDetails() (status string, details map[string]interface{}) {
	const contextTimeoutDuration = 50

	status = datasource.StatusUp

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeoutDuration*time.Millisecond)
	defer cancel()

	it := g.client.Topics(ctx)

	details = make(map[string]interface{})

	for {
		topic, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}

		if err != nil {
			status = datasource.StatusDown

			break
		}

		if topic != nil {
			details[topic.ID()] = topic
		}
	}

	return status, details
}

//nolint:dupl // getReaderDetails provides the subscription details for current google subscriptions.
func (g *googleClient) getReaderDetails() (status string, details map[string]interface{}) {
	const contextTimeoutDuration = 50

	status = datasource.StatusUp

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeoutDuration*time.Millisecond)
	defer cancel()

	subIt := g.client.Subscriptions(ctx)

	details = make(map[string]interface{})

	for {
		subscription, err := subIt.Next()
		if errors.Is(err, iterator.Done) {
			break
		}

		if err != nil {
			status = datasource.StatusDown

			break
		}

		if subscription != nil {
			details[subscription.ID()] = subscription
		}
	}

	return status, details
}
