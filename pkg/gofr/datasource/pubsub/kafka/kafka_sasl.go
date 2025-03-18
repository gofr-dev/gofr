package kafka

import (
	"fmt"
	"strings"

	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

func setDefaultSecurityProtocol(conf *Config) {
	if conf.SecurityProtocol == "" {
		conf.SecurityProtocol = protocolPlainText
	}
}

func validateSecurityProtocol(conf *Config) error {
	protocol := strings.ToUpper(conf.SecurityProtocol)

	switch protocol {
	case protocolPlainText, protocolSASL, protocolSASLSSL, protocolSSL:
		return nil
	default:
		return fmt.Errorf("unsupported security protocol: %s: %w", protocol, errUnsupportedSecurityProtocol)
	}
}

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

func validateSASLConfigs(conf *Config) error {
	protocol := strings.ToUpper(conf.SecurityProtocol)

	if protocol == protocolSASL || protocol == protocolSASLSSL {
		if conf.SASLMechanism == "" || conf.SASLUser == "" || conf.SASLPassword == "" {
			return fmt.Errorf("SASL credentials missing: %w", errSASLCredentialsMissing)
		}
	}

	return nil
}
