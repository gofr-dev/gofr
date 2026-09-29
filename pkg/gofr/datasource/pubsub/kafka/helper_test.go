package kafka

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/logging"
)

func TestKafkaClient_retryConnect_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	k := &kafkaClient{config: Config{Brokers: []string{"localhost:9092"}}}

	// A canceled context makes retryConnect return before its first retry tick.
	k.retryConnect(ctx)

	assert.Nil(t, k.conn)
}

func TestSetupDialer(t *testing.T) {
	missingCA := filepath.Join(t.TempDir(), "missing-ca.pem")

	tests := []struct {
		desc    string
		config  Config
		expErr  error
		expSASL bool
		expTLS  bool
	}{
		{
			desc:   "plaintext dialer",
			config: Config{SecurityProtocol: protocolPlainText},
		},
		{
			desc:    "SASL plaintext with plain mechanism",
			config:  Config{SecurityProtocol: protocolSASL, SASLMechanism: "PLAIN", SASLUser: "u", SASLPassword: "p"},
			expSASL: true,
		},
		{
			desc:   "SASL with unsupported mechanism",
			config: Config{SecurityProtocol: protocolSASL, SASLMechanism: "UNKNOWN"},
			expErr: errUnsupportedSASLMechanism,
		},
		{
			desc:   "SSL with insecure skip verify",
			config: Config{SecurityProtocol: protocolSSL, TLS: TLSConfig{InsecureSkipVerify: true}},
			expTLS: true,
		},
		{
			desc: "SASL SSL with plain mechanism",
			config: Config{SecurityProtocol: protocolSASLSSL, SASLMechanism: "PLAIN", SASLUser: "u", SASLPassword: "p",
				TLS: TLSConfig{InsecureSkipVerify: true}},
			expSASL: true,
			expTLS:  true,
		},
		{
			desc:   "SSL with unreadable CA file",
			config: Config{SecurityProtocol: protocolSSL, TLS: TLSConfig{CACertFile: missingCA}},
			expErr: errCACertFileRead,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			dialer, err := setupDialer(&tc.config)

			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expErr == nil, dialer != nil)
			assert.Equal(t, tc.expSASL, dialer != nil && dialer.SASLMechanism != nil)
			assert.Equal(t, tc.expTLS, dialer != nil && dialer.TLS != nil)
		})
	}
}

func TestKafkaClient_reconnectAdminLocked_DialerSetupError(t *testing.T) {
	existing := &multiConn{}

	k := &kafkaClient{
		config: Config{Brokers: []string{"localhost:9092"}, SecurityProtocol: protocolSASL, SASLMechanism: "UNKNOWN"},
		conn:   existing,
		logger: logging.NewMockLogger(logging.DEBUG),
	}

	err := k.reconnectAdminLocked(t.Context())

	require.ErrorIs(t, err, errUnsupportedSASLMechanism)
	assert.Same(t, existing, k.conn)
	assert.Nil(t, k.dialer)
}

func TestConnectToBrokers(t *testing.T) {
	reachable := newClosingListener(t).String()
	unreachable := net.JoinHostPort("127.0.0.1", "0")

	tests := []struct {
		desc     string
		brokers  []string
		expErr   error
		expConns int
	}{
		{desc: "no brokers", expErr: errBrokerNotProvided},
		{desc: "all brokers unreachable", brokers: []string{unreachable}, expErr: errFailedToConnectBrokers},
		{desc: "one of two brokers reachable", brokers: []string{unreachable, reachable}, expConns: 1},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			conns, err := connectToBrokers(t.Context(), tc.brokers, &kafka.Dialer{Timeout: 5 * time.Second},
				logging.NewMockLogger(logging.DEBUG))

			t.Cleanup(func() { _ = (&multiConn{conns: conns}).Close() })

			require.ErrorIs(t, err, tc.expErr)
			assert.Len(t, conns, tc.expConns)
		})
	}
}

func TestKafkaClient_readMessages_NoMessages(t *testing.T) {
	expired, cancelExpired := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancelExpired()

	tests := []struct {
		desc      string
		ctx       context.Context
		limit     int
		expResult []byte
	}{
		{desc: "zero limit reads nothing", ctx: t.Context(), limit: 0},
		{desc: "deadline exceeded is treated as end of messages", ctx: expired, limit: 5},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			reader := kafka.NewReader(kafka.ReaderConfig{
				Brokers: []string{net.JoinHostPort("127.0.0.1", "0")},
				Topic:   "test-topic",
			})

			t.Cleanup(func() { _ = reader.Close() })

			k := &kafkaClient{}

			result, err := k.readMessages(tc.ctx, reader, tc.limit)

			require.NoError(t, err)
			assert.Equal(t, tc.expResult, result)
		})
	}
}
