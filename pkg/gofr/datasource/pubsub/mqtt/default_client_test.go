package mqtt

import (
	"testing"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/mock/gomock"
)

func TestRetryDefaultConnect_ConnectsOnFirstAttempt(t *testing.T) {
	tests := []struct {
		desc     string
		config   *Config
		clientID string
	}{
		{
			desc:     "connects on first attempt",
			config:   &Config{Hostname: "localhost", Port: 8883},
			clientID: "custom-client",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockClient := NewMockClient(ctrl)
			mockToken := NewMockToken(ctrl)
			mockLogger := NewMockLogger(ctrl)

			mockClient.EXPECT().Connect().Return(mockToken).Times(1)
			mockToken.EXPECT().Wait().Return(true)
			mockToken.EXPECT().Error().Return(nil)
			mockLogger.EXPECT().Infof("connected to MQTT at '%v:%v' with clientID '%v'",
				tc.config.Hostname, tc.config.Port, tc.clientID)

			opts := mqtt.NewClientOptions()
			opts.SetClientID(tc.clientID)

			retryDefaultConnect(mockClient, tc.config, mockLogger, opts)
		})
	}
}
