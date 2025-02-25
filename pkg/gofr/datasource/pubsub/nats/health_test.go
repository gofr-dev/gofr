package nats

import (
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/logging"
)

const (
	NATSServer = "nats://localhost:4222"
)

func TestNATSClient_Health(t *testing.T) {
	testCases := defineHealthTestCases()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runHealthTestCase(t, tc)
		})
	}
}

func defineHealthTestCases() []healthTestCase {
	return []healthTestCase{
		{
			name: "HealthyConnection",
			setupMocks: func(mockConnManager *MockConnectionManagerInterface, mockJS *MockJetStream) {
				mockConnManager.EXPECT().Health().Return(datasource.Health{
					Status: datasource.StatusUp,
					Details: map[string]any{
						"host":              NATSServer,
						"connection_status": jetStreamConnected,
					},
				})
				mockConnManager.EXPECT().JetStream().Return(mockJS, nil)
				mockJS.EXPECT().AccountInfo(gomock.Any()).Return(&jetstream.AccountInfo{}, nil)
			},
			expectedStatus: datasource.StatusUp,
			expectedDetails: map[string]any{
				"host":              NATSServer,
				"backend":           natsBackend,
				"connection_status": jetStreamConnected,
				"jetstream_enabled": true,
				"jetstream_status":  jetStreamStatusOK,
			},
		},
		{
			name: "DisconnectedStatus",
			setupMocks: func(mockConnManager *MockConnectionManagerInterface, _ *MockJetStream) {
				mockConnManager.EXPECT().Health().Return(datasource.Health{
					Status: datasource.StatusDown,
					Details: map[string]any{
						"host":              NATSServer,
						"connection_status": jetStreamDisconnecting,
					},
				})
				mockConnManager.EXPECT().JetStream().Return(nil, errJetStreamNotConfigured)
			},
			expectedStatus: datasource.StatusDown,
			expectedDetails: map[string]any{
				"host":              NATSServer,
				"backend":           natsBackend,
				"connection_status": jetStreamDisconnecting,
				"jetstream_enabled": false,
				"jetstream_status":  jetStreamStatusError + ": jStream is not configured",
			},
		},
		{
			name: "JetStreamError",
			setupMocks: func(mockConnManager *MockConnectionManagerInterface, mockJS *MockJetStream) {
				mockConnManager.EXPECT().Health().Return(datasource.Health{
					Status: datasource.StatusUp,
					Details: map[string]any{
						"host":              NATSServer,
						"connection_status": jetStreamConnected,
					},
				})
				mockConnManager.EXPECT().JetStream().Return(mockJS, nil)
				mockJS.EXPECT().AccountInfo(gomock.Any()).Return(nil, errJetStream)
			},
			expectedStatus: datasource.StatusUp,
			expectedDetails: map[string]any{
				"host":              NATSServer,
				"backend":           natsBackend,
				"connection_status": jetStreamConnected,
				"jetstream_enabled": true,
				"jetstream_status":  jetStreamStatusError + ": " + errJetStream.Error(),
			},
		},
	}
}

func runHealthTestCase(t *testing.T, tc healthTestCase) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConnManager := NewMockConnectionManagerInterface(ctrl)
	mockJS := NewMockJetStream(ctrl)

	tc.setupMocks(mockConnManager, mockJS)

	client := &Client{
		connManager: mockConnManager,
		Config:      &Config{Server: NATSServer},
		logger:      logging.NewMockLogger(logging.DEBUG),
	}

	h := client.Health()

	assert.Equal(t, tc.expectedStatus, h.Status)
	assert.Equal(t, tc.expectedDetails, h.Details)
}

type healthTestCase struct {
	name            string
	setupMocks      func(*MockConnectionManagerInterface, *MockJetStream)
	expectedStatus  string
	expectedDetails map[string]any
}
