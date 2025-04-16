package kafka

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"gofr.dev/pkg/gofr/datasource"
)

func (k *kafkaClient) Health() datasource.Health {
	health := datasource.Health{
		Status:  datasource.StatusDown, // Default to DOWN
		Details: make(map[string]any),
	}

	if k.conn == nil {
		health.Details["error"] = "invalid connection type"
		return health
	}

	k.conn.mu.RLock()
	defer k.conn.mu.RUnlock()

	var (
		brokerStatus   = make([]map[string]any, 0)
		allDown        = true
		controllerAddr string
	)

	for _, conn := range k.conn.conns {
		if conn == nil {
			continue
		}

		brokerAddr := conn.RemoteAddr().String()
		status := map[string]any{
			"broker":       brokerAddr,
			"status":       "DOWN",
			"isController": false,
			"error":        nil,
		}

		// Lightweight liveness check using metadata request
		_, err := conn.ReadPartitions()
		if err != nil {
			status["error"] = err.Error()
			brokerStatus = append(brokerStatus, status)

			continue
		}

		// Broker is reachable
		status["status"] = "UP"
		allDown = false

		// Get controller address once from first responsive broker
		if controllerAddr == "" {
			controller, err := conn.Controller()
			if err != nil {
				status["error"] = fmt.Sprintf("controller lookup failed: %v", err)
			} else if controller.Host != "" {
				controllerAddr = net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port))
			}
		}

		// Check controller status
		status["isController"] = brokerAddr == controllerAddr

		brokerStatus = append(brokerStatus, status)
	}

	if !allDown {
		health.Status = datasource.StatusUp
	}

	health.Details["brokers"] = brokerStatus
	health.Details["backend"] = "KAFKA"
	health.Details["writer"] = k.getWriterStatsAsMap()
	health.Details["readers"] = k.getReaderStatsAsMap()

	return health
}

func (k *kafkaClient) getReaderStatsAsMap() []any {
	readerStats := make([]any, 0)

	for _, reader := range k.reader {
		var readerStat map[string]any
		if err := convertStructToMap(reader.Stats(), &readerStat); err != nil {
			k.logger.Errorf("kafka Reader Stats processing failed: %v", err)
			continue // Log the error but continue processing other readers
		}

		readerStats = append(readerStats, readerStat)
	}

	return readerStats
}

func (k *kafkaClient) getWriterStatsAsMap() map[string]any {
	writerStats := make(map[string]any)

	if err := convertStructToMap(k.writer.Stats(), &writerStats); err != nil {
		k.logger.Errorf("kafka Writer Stats processing failed: %v", err)

		return nil
	}

	return writerStats
}

// convertStructToMap tries to convert any struct to a map representation by first marshaling it to JSON, then unmarshalling into a map.
func convertStructToMap(input, output any) error {
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
