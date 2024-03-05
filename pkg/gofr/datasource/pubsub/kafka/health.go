package kafka

import "gofr.dev/pkg/gofr/datasource"

func (k *kafkaClient) Health() (health datasource.Health) {
	health = datasource.Health{Details: make(map[string]interface{})}

	health.Status = "UP"

	_, err := k.conn.Controller()
	if err != nil {
		health.Status = "DOWN"
	}

	health.Details["host"] = k.config.Broker
	health.Details["backend"] = "KAFKA"
	health.Details["writer"] = k.getWriterStatsAsMap()
	health.Details["readers"] = k.getReaderStatsAsMap()

	return
}
