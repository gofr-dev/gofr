package google

import (
	"bytes"
	"context"
	"time"

	gcPubSub "cloud.google.com/go/pubsub"
)

func validateConfigs(conf *Config) error {
	if conf.ProjectID == "" {
		return errProjectIDNotProvided
	}

	if conf.SubscriptionName == "" {
		return errSubscriptionNotProvided
	}

	return nil
}

func parseQueryArgs(args ...any) (time.Duration, int) {
	timeout := defaultQueryTimeout
	limit := defaultMessageLimit

	if len(args) > 1 {
		if val, ok := args[1].(int); ok {
			limit = val
		}
	}

	return timeout, limit
}

func (g *googleClient) getQuerySubscription(ctx context.Context, topic *gcPubSub.Topic) (*gcPubSub.Subscription, error) {
	subName := g.SubscriptionName + "-query-" + topic.ID()
	subscription := g.client.Subscription(subName)

	exists, err := subscription.Exists(ctx)
	if err != nil {
		return nil, err
	}

	if !exists {
		subscription, err = g.client.CreateSubscription(ctx, subName, gcPubSub.SubscriptionConfig{
			Topic: topic,
		})
		if err != nil {
			return nil, err
		}
	}

	return subscription, nil
}

func (g *googleClient) collectMessages(ctx context.Context, msgChan <-chan []byte, limit int) []byte {
	var result bytes.Buffer

	collected := 0

	for {
		if limit > 0 && collected >= limit {
			break
		}

		select {
		case msg, ok := <-msgChan:
			if !ok {
				g.logger.Debugf("Query: message channel closed, collected %d messages", collected)

				return result.Bytes()
			}

			if result.Len() > 0 {
				result.WriteByte('\n')
			}

			result.Write(msg)

			collected++

			g.logger.Debugf("Query: collected message %d", collected)

		case <-ctx.Done():
			return result.Bytes()
		}
	}

	return result.Bytes()
}
