package kafka

import (
	"encoding/json"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/health"
)

func (k *kafkaClient) Health() (h health.Health) {
	clientHealth := health.Health{Details: make(map[string]interface{})}

	clientHealth.Status = datasource.StatusUp

	_, err := k.conn.Controller()
	if err != nil {
		clientHealth.Status = datasource.StatusDown
	}

	clientHealth.Details["host"] = k.config.Broker
	clientHealth.Details["backend"] = "KAFKA"
	clientHealth.Details["writers"] = k.getWriterStatsAsMap()
	clientHealth.Details["readers"] = k.getReaderStatsAsMap()

	return clientHealth
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
