package mqtt

import (
	"bytes"
	"gofr.dev/pkg"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/gofr/types"
	"io"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
)

func Test_New(t *testing.T) {
	b := new(bytes.Buffer)
	mockLogger := log.NewMockLogger(b)

	testCases := []struct {
		desc        string
		config      *Config
		expectedLog string
		expErr      error
	}{
		{
			desc: "successful connection",
			config: &Config{
				Protocol:                "tcp",
				Hostname:                "localhost",
				Port:                    8883,
				ClientID:                "test-id",
				Topic:                   "topic1",
				ConnectionRetryDuration: 10,
			},
			expectedLog: "connected to MQTT",
			expErr:      nil,
		},
		{
			desc: "unsuccessful connection",
			config: &Config{
				Protocol:                "tcp",
				Hostname:                "somehost",
				Username:                "test-user",
				Password:                "test-pass",
				Port:                    8883,
				ClientID:                "test-id",
				Topic:                   "topic1",
				ConnectionRetryDuration: 10,
			},
			expectedLog: "connected to MQTT",
			expErr:      errors.Error("network Error : dial tcp: lookup somehost: no such host"),
		},
	}

	for _, tc := range testCases {
		_, err := New(tc.config, mockLogger)

		assert.Contains(t, b.String(), tc.expectedLog, "TESTCASE FAILED \nexpected: %v, \ngot: %v", tc.expectedLog, b.String())

		if err != nil {
			assert.Equal(t, tc.expErr.Error(), err.Error(), "TESTCASE FAILED \nexpected: %v, \ngot: %v", tc.expErr, err)
		}
	}
}

func Test_PublishEvent(t *testing.T) {
	mockLogger := log.NewMockLogger(io.Discard)

	testCases := []struct {
		desc   string
		config *Config
		expErr error
	}{
		{
			desc: "successful publish",
			config: &Config{
				Protocol:                "tcp",
				Hostname:                "localhost",
				Port:                    8883,
				ClientID:                "test-id",
				Topic:                   "topic1",
				ConnectionRetryDuration: 10,
			},
			expErr: nil,
		},
		{
			desc: "unsuccessful publish",
			config: &Config{
				Protocol:                "tcp",
				Hostname:                "localhost",
				Port:                    8823,
				ClientID:                "test-id",
				Topic:                   "topic2",
				ConnectionRetryDuration: 10,
			},
			expErr: errors.Error("client not configured"),
		},
	}

	for i, tc := range testCases {
		m, _ := New(tc.config, mockLogger)

		err := m.PublishEvent("", "test-msg", nil)
		if err != nil && tc.expErr.Error() != err.Error() {
			t.Errorf("TESTCASE [%d] FAILED\n Expected: %v\n Got: %v", i, tc.expErr, err)
		}
	}
}

func Test_IsSet(t *testing.T) {
	m, _ := New(&Config{
		Protocol:                "tcp",
		Hostname:                "localhost",
		Port:                    8883,
		ClientID:                "test-id",
		Topic:                   "topic1",
		ConnectionRetryDuration: 10,
	}, log.NewMockLogger(io.Discard))

	var mq *MQTT

	testCases := []struct {
		m   pubsub.PublisherSubscriber
		exp bool
	}{
		{mq, false},
		{&MQTT{}, false},
		{m, true},
	}

	for i, tc := range testCases {
		out := tc.m.IsSet()

		if out != tc.exp {
			t.Errorf("TESTCASE [%d] FAILED\n Expected: %v\n Got: %v", i, tc.exp, out)
		}
	}
}

func Test_HealthCheck(t *testing.T) {
	testcases := []struct {
		c    Config
		resp types.Health
	}{
		{
			c: Config{
				Protocol:                "tcp",
				Hostname:                "localhost",
				Port:                    8883,
				ClientID:                "test-id",
				Topic:                   "topic1",
				ConnectionRetryDuration: 10,
			},
			resp: types.Health{
				Name: datastore.Mqtt, Status: pkg.StatusUp, Host: "localhost", Database: "topic1"}},
		{
			c: Config{
				Protocol:                "tcp",
				Hostname:                "localhost",
				Port:                    8823,
				ClientID:                "test-id",
				Topic:                   "topic1",
				ConnectionRetryDuration: 10,
			},
			resp: types.Health{Name: datastore.Mqtt, Status: pkg.StatusDown, Host: "localhost", Database: "topic1"}},
	}

	for i, v := range testcases {
		testConfig := v.c
		conn, _ := New(&testConfig, log.NewMockLogger(io.Discard))

		resp := conn.HealthCheck()
		if !reflect.DeepEqual(resp, v.resp) {
			t.Errorf("[TESTCASE%d]Failed. Got %v\tExpected %v\n", i+1, resp, v.resp)
		}
	}
}

func Test_HealthCheckFailure(t *testing.T) {
	var m *MQTT
	exp := types.Health{Name: datastore.Mqtt, Status: pkg.StatusDown}
	out := m.HealthCheck()

	if !reflect.DeepEqual(exp, out) {
		t.Errorf("TESTCASE FAILED\n Got: %v\n Expected: %v", out, exp)
	}
}

func Test_Ping(t *testing.T) {

}

func Test_Bind(t *testing.T) {
	var k struct {
		Test string `json:"test"`
	}

	m := MQTT{}

	err := m.Bind([]byte(`{"test":"test"}`), &k)
	if err != nil {
		t.Error("Test case failed", err)
	}
}
