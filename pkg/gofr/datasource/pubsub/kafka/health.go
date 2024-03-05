package kafka

import "gofr.dev/pkg/gofr/datasource"

func (k *kafkaClient) Health() (health datasource.Health) {
	health = datasource.Health{Details: make(map[string]interface{})}

	health.Status = datasource.StatusUp

	_, err := k.conn.Controller()
	if err != nil {
		health.Status = datasource.StatusDown
	}

	health.Details["host"] = k.config.Broker
	health.Details["backend"] = "KAFKA"
	health.Details["writers"] = k.getWriterStatsAsMap()
	health.Details["readers"] = k.getReaderStatsAsMap()

	return
}
