package kafka

import (
	"fmt"
	"strings"

	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

func getSASLMechanism(mechanism, username, password string) (sasl.Mechanism, error) {
	switch strings.ToUpper(mechanism) {
	case "PLAIN":
		return plain.Mechanism{
			Username: username,
			Password: password,
		}, nil
	case "SCRAM-SHA-256":
		mechanism, _ := scram.Mechanism(scram.SHA256, username, password)

		return mechanism, nil
	case "SCRAM-SHA-512":
		mechanism, _ := scram.Mechanism(scram.SHA512, username, password)

		return mechanism, nil
	default:
		return nil, fmt.Errorf("%w: %s", errUnsupportedSASLMechanism, mechanism)
	}
}
