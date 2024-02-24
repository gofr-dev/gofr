package kafka

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_validateConfigs(t *testing.T) {
	config := Config{Broker: "kafkabroker", ConsumerGroupID: "1"}

	err := validateConfigs(config)

	assert.Nil(t, err)
}

func Test_validateConfigsErrConsumerGroupNotFound(t *testing.T) {
	config := Config{Broker: "kafkabroker"}

	err := validateConfigs(config)

	assert.Equal(t, errConsumerGroupNotProvided, err)
}

func Test_validateConfigsErrBrokerNotProvided(t *testing.T) {
	config := Config{ConsumerGroupID: "1"}

	err := validateConfigs(config)

	assert.Equal(t, err, errBrokerNotProvided)
}
