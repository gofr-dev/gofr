package nats

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
	"gofr.dev/pkg/gofr/datasource"
)

func (n *natsClient) Health() datasource.Health {
	health := datasource.Health{
		Details: make(map[string]interface{}),
	}

	health.Status = datasource.StatusUp

	// Check connection status
	if n.conn.Status() != nats.CONNECTED {
		health.Status = datasource.StatusDown
	}

	health.Details["host"] = n.config.Server
	health.Details["backend"] = "NATS"
	health.Details["connection_status"] = n.conn.Status().String()
	health.Details["jetstream_enabled"] = n.js != nil

	// Get JetStream information if available
	if n.js != nil {
		jsInfo, err := n.getJetStreamInfo()
		if err != nil {
			n.logger.Errorf("Failed to get JetStream info: %v", err)
		} else {
			health.Details["jetstream_info"] = jsInfo
		}
	}

	return health
}

func (n *natsClient) getJetStreamInfo() (map[string]interface{}, error) {
	jsInfo, err := n.js.AccountInfo(n.conn.Opts.Context)
	if err != nil {
		return nil, err
	}

	info := make(map[string]interface{})
	err = convertStructToMap(jsInfo, &info)
	if err != nil {
		return nil, err
	}

	return info, nil
}

// convertStructToMap tries to convert any struct to a map representation by first marshaling it to JSON, then unmarshalling into a map.
func convertStructToMap(input, output interface{}) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, &output)
	if err != nil {
		return err
	}

	return nil
}
