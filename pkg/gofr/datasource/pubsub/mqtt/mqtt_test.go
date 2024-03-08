package mqtt

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/testutil"
)

func TestMQTT_New(t *testing.T) {
	var client *MQTT

	conf := Config{
		Protocol:         "tcp",
		Hostname:         "localhost",
		Port:             1883,
		QoS:              0,
		Order:            false,
		RetrieveRetained: false,
	}

	out := testutil.StderrOutputForFunc(func() {
		mockLogger := testutil.NewMockLogger(testutil.ERRORLOG)
		client = New(&conf, mockLogger, nil)
	})

	assert.Nil(t, client.Client)
	assert.Contains(t, out, "cannot connect to MQTT")
}

func TestMQTT_New_InvalidConfigs(t *testing.T) {
	var client *MQTT

	out := testutil.StderrOutputForFunc(func() {
		mockLogger := testutil.NewMockLogger(testutil.ERRORLOG)
		client = New(&Config{}, mockLogger, nil)
	})

	assert.Nil(t, client)
	assert.Contains(t, out, "could not initialize MQTT")
}

func TestMQTT_validateConfigs(t *testing.T) {
	testCase := []struct {
		desc        string
		config      *Config
		expectedErr error
	}{
		{desc: "missing protocol", config: &Config{}, expectedErr: errProtocolNotProvided},
		{desc: "missing hostname", config: &Config{Protocol: "tcp"}, expectedErr: errHostNotProvided},
		{desc: "invalid port", config: &Config{Protocol: "tcp", Hostname: "localhost"}, expectedErr: errInvalidPort},
	}

	for _, tc := range testCase {
		err := validateConfigs(tc.config)

		assert.Equal(t, tc.expectedErr, err)
	}
}

func TestMQTT_getMQTTClientOptions(t *testing.T) {
	conf := Config{
		Protocol: "tcp",
		Hostname: "localhost",
		Port:     1883,
		QoS:      0,
		Username: "user",
		Password: "pass",
		ClientID: "test",
		Order:    false,
	}

	expectedURL, _ := url.Parse("tcp://localhost:1883")
	options := getMQTTClientOptions(&conf, nil)

	assert.Equal(t, options.ClientID, conf.ClientID)
	assert.ElementsMatch(t, options.Servers, []*url.URL{expectedURL})
	assert.Equal(t, options.Username, conf.Username)
	assert.Equal(t, options.Password, conf.Password)
	assert.Equal(t, options.Order, conf.Order)
}

func TestMQTT_getMQTTClientOptions_ClientIDWarning(t *testing.T) {
	conf := Config{
		Protocol: "tcp",
		Hostname: "localhost",
		Port:     1883,
	}

	out := testutil.StdoutOutputForFunc(func() {
		mockLogger := testutil.NewMockLogger(testutil.WARNLOG)
		_ = getMQTTClientOptions(&conf, mockLogger)
	})

	assert.Contains(t, out, "client id not provided, please provide a clientID to prevent unexpected behaviors")
}
