package kafka

import (
	"net"
	"strconv"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
)

// newClosingListener starts a loopback TCP listener that accepts connections and closes
// them immediately, so a kafka dial succeeds but any subsequent request fails.
func newClosingListener(t *testing.T) *net.TCPAddr {
	t.Helper()

	lc := net.ListenConfig{}

	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}

			_ = conn.Close()
		}
	}()

	return ln.Addr().(*net.TCPAddr)
}

// unusedLoopbackPort returns a loopback port that nothing is listening on.
func unusedLoopbackPort(t *testing.T) int {
	t.Helper()

	lc := net.ListenConfig{}

	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	port := ln.Addr().(*net.TCPAddr).Port

	require.NoError(t, ln.Close())

	return port
}

func TestKafkaClient_AdminMethods_NotConnected(t *testing.T) {
	k := &kafkaClient{}

	_, controllerErr := k.Controller()

	tests := []struct {
		desc   string
		err    error
		expErr error
	}{
		{desc: "controller", err: controllerErr, expErr: errClientNotConnected},
		{desc: "create topic", err: k.CreateTopic(t.Context(), "topic"), expErr: errClientNotConnected},
		{desc: "delete topic", err: k.DeleteTopic(t.Context(), "topic"), expErr: errClientNotConnected},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			require.ErrorIs(t, tc.err, tc.expErr)
		})
	}
}

func TestMultiConn_Controller(t *testing.T) {
	ctrl := gomock.NewController(t)

	failing := NewMockConnection(ctrl)
	healthy := NewMockConnection(ctrl)

	tests := []struct {
		desc      string
		conns     []Connection
		mockCall  func()
		expBroker kafka.Broker
		expErr    error
	}{
		{
			desc:     "no connections",
			mockCall: func() {},
			expErr:   errNoActiveConnections,
		},
		{
			desc:  "skips nil and failing connections",
			conns: []Connection{nil, failing, healthy},
			mockCall: func() {
				failing.EXPECT().Controller().Return(kafka.Broker{}, errNotController)
				healthy.EXPECT().Controller().Return(kafka.Broker{Host: "localhost", Port: 9092}, nil)
			},
			expBroker: kafka.Broker{Host: "localhost", Port: 9092},
		},
		{
			desc:  "all connections fail",
			conns: []Connection{nil, failing},
			mockCall: func() {
				failing.EXPECT().Controller().Return(kafka.Broker{}, errNotController)
			},
			expErr: errNoActiveConnections,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			tc.mockCall()

			m := &multiConn{conns: tc.conns}

			broker, err := m.Controller()

			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expBroker, broker)
		})
	}
}

func TestMultiConn_TopicAdmin_ControllerLookup(t *testing.T) {
	ctrl := gomock.NewController(t)

	tests := []struct {
		desc     string
		broker   kafka.Broker
		call     func(m *multiConn) error
		expErr   string
		expConns int
	}{
		{
			desc:     "create topics with invalid controller port",
			broker:   kafka.Broker{Host: "127.0.0.1", Port: -1},
			call:     func(m *multiConn) error { return m.CreateTopics(kafka.TopicConfig{Topic: "t"}) },
			expErr:   "invalid port",
			expConns: 2,
		},
		{
			desc:     "delete topics with invalid controller port",
			broker:   kafka.Broker{Host: "127.0.0.1", Port: -1},
			call:     func(m *multiConn) error { return m.DeleteTopics("t") },
			expErr:   "invalid port",
			expConns: 2,
		},
		{
			desc:     "create topics fails dialing unknown controller",
			broker:   kafka.Broker{Host: "127.0.0.1", Port: unusedLoopbackPort(t)},
			call:     func(m *multiConn) error { return m.CreateTopics(kafka.TopicConfig{Topic: "t"}) },
			expErr:   "failed to dial",
			expConns: 2,
		},
		{
			desc:     "delete topics fails dialing unknown controller",
			broker:   kafka.Broker{Host: "127.0.0.1", Port: unusedLoopbackPort(t)},
			call:     func(m *multiConn) error { return m.DeleteTopics("t") },
			expErr:   "failed to dial",
			expConns: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			conn := NewMockConnection(ctrl)

			conn.EXPECT().Controller().Return(tc.broker, nil)
			conn.EXPECT().RemoteAddr().Return(&net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 9092}).AnyTimes()

			m := &multiConn{conns: []Connection{conn, nil}, dialer: &kafka.Dialer{}}

			err := tc.call(m)

			require.ErrorContains(t, err, tc.expErr)
			assert.Len(t, m.conns, tc.expConns)
		})
	}
}

func TestMultiConn_TopicAdmin_DialsNewController(t *testing.T) {
	ctrl := gomock.NewController(t)

	addr := newClosingListener(t)
	broker := kafka.Broker{Host: addr.IP.String(), Port: addr.Port}

	tests := []struct {
		desc string
		call func(m *multiConn) error
	}{
		{
			desc: "create topics",
			call: func(m *multiConn) error { return m.CreateTopics(kafka.TopicConfig{Topic: "t"}) },
		},
		{
			desc: "delete topics",
			call: func(m *multiConn) error { return m.DeleteTopics("t") },
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			existing := NewMockConnection(ctrl)

			existing.EXPECT().Controller().Return(broker, nil)
			existing.EXPECT().RemoteAddr().Return(&net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 9092})

			m := &multiConn{conns: []Connection{existing}, dialer: &kafka.Dialer{}}

			// The listener drops the connection, so the request on the freshly dialed
			// controller connection fails.
			err := tc.call(m)

			require.Error(t, err)
			assert.Equal(t, net.JoinHostPort(broker.Host, strconv.Itoa(broker.Port)), m.conns[1].RemoteAddr().String())

			_ = m.conns[1].Close()
		})
	}
}

func TestKafkaClient_initialize(t *testing.T) {
	ctrl := gomock.NewController(t)

	existingReader := NewMockReader(ctrl)
	existingReader.EXPECT().Close().Return(nil)

	existingReaders := map[string]Reader{"existing": existingReader}

	tests := []struct {
		desc       string
		config     Config
		reader     map[string]Reader
		expErr     error
		expConns   int
		expReaders map[string]Reader
	}{
		{
			desc:   "dialer setup fails",
			config: Config{Brokers: []string{"localhost:9092"}, SecurityProtocol: protocolSASL, SASLMechanism: "UNKNOWN"},
			expErr: errUnsupportedSASLMechanism,
		},
		{
			desc:       "creates reader map when none exists",
			config:     Config{Brokers: []string{"localhost:9092"}, BatchSize: 1, BatchBytes: 1, BatchTimeout: 1},
			expConns:   1,
			expReaders: map[string]Reader{},
		},
		{
			desc:       "keeps readers created before initialize",
			config:     Config{Brokers: []string{"localhost:9092"}, BatchSize: 1, BatchBytes: 1, BatchTimeout: 1},
			reader:     existingReaders,
			expConns:   1,
			expReaders: existingReaders,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			stubConnectToBrokers(t, []Connection{&MockConn{addr: "127.0.0.1:9092", isHealthy: true}})

			k := &kafkaClient{config: tc.config, reader: tc.reader, logger: logging.NewMockLogger(logging.DEBUG)}

			err := k.initialize(t.Context())

			t.Cleanup(func() { _ = k.Close() })

			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expReaders, k.reader)
			assert.Equal(t, tc.expConns, connCount(k.conn))
		})
	}
}

// connCount returns the number of admin connections held by m, treating nil as zero.
func connCount(m *multiConn) int {
	if m == nil {
		return 0
	}

	return len(m.conns)
}
