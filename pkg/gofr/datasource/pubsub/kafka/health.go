package kafka

import (
	"encoding/json"

	"gofr.dev/pkg/gofr/datasource"
)

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

	return health
}

func (k *kafkaClient) getReaderStatsAsMap() []interface{} {
	readerStats := make([]interface{}, 0)

	for _, reader := range k.reader {
		var readerStat map[string]interface{}
		if err := convertStructToMap(reader.Stats(), &readerStat); err != nil {
			k.logger.Errorf("kafka Reader Stats processing failed: %v", err)
			continue // Log the error but continue processing other readers
		}

		readerStats = append(readerStats, readerStat)
	}

	return readerStats
}

func (k *kafkaClient) getWriterStatsAsMap() map[string]interface{} {
	writerStats := make(map[string]interface{})

	if err := convertStructToMap(k.writer.Stats(), &writerStats); err != nil {
		k.logger.Errorf("kafka Writer Stats processing failed: %v", err)

		return nil
	}

	return writerStats
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
