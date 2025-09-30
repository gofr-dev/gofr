package nats

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDefaultNATSConnector_Connect(t *testing.T) {
	// Start a NATS server
	ns, url := startNATSServer(t)
	defer ns.Shutdown()

	connector := &defaultConnector{}

	// Test successful connection
	conn, err := connector.Connect(url)
	require.NoError(t, err)
	assert.NotNil(t, conn)
	assert.Implements(t, (*ConnInterface)(nil), conn)

	// Close the connection
	conn.Close()

	// Test connection failure
	_, err = connector.Connect("nats://invalid-url:4222")
	assert.Error(t, err)
}

func TestDefaultJetStreamCreator_New(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("Successful jStream creation", func(t *testing.T) {
		// Start a NATS server
		ns, url := startNATSServer(t)
		defer ns.Shutdown()

		// Create a real NATS connection
		nc, err := nats.Connect(url)
		require.NoError(t, err)

		defer nc.Close()

		// Wrap the real connection
		wrapper := &natsConnWrapper{conn: nc}

		creator := &DefaultJetStreamCreator{}

		// Test successful jStream creation
		js, err := creator.New(wrapper)
		require.NoError(t, err)
		assert.NotNil(t, js)
	})

	t.Run("jStream creation failure", func(t *testing.T) {
		// Create a mock NATS connection
		mockConn := NewMockConnInterface(ctrl)

		// Mock the jStream method to return an error
		expectedError := errJetStreamCreationFailed
		mockConn.EXPECT().JetStream().Return(nil, expectedError)

		creator := &DefaultJetStreamCreator{}

		// Test jStream creation failure
		js, err := creator.New(mockConn)
		require.Error(t, err)
		assert.Nil(t, js)
		assert.Equal(t, expectedError, err)
	})
}
