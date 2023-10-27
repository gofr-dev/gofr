//go:build !skip

package gofr

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/log"
	"io"
	"strings"
	"testing"
)

func Test_CQL_initialize(t *testing.T) {
	// this is done to so that it doesn't affect the other tests related to cassandra
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	config.NewGoDotEnvProvider(logger, "../../configs")

	tcs := []struct {
		configLoc   Config
		expectedStr string
	}{
		{mockConfig{}, "Connected to cassandra"},
		{configLoc: &config.MockConfig{Data: map[string]string{"CASS_DB_HOST": "cassandra", "CASS_DB_PORT": "2003"}}},
	}

	for _, tc := range tcs {
		k := &Gofr{}
		k.Logger = logger

		initializeCassandra(mockConfig{}, k)

		if !strings.Contains(b.String(), tc.expectedStr) {
			t.Errorf("FAILED, expected: `%v` in the logs, got: %v", tc.expectedStr, b.String())
		}
	}
}

func Test_CQL_InitializeCassandraFromConfigs(t *testing.T) {
	var cfg mockConfig

	logger := log.NewMockLogger(io.Discard)

	conn, err := InitializeCassandraFromConfigs(cfg, logger, "")
	if err != nil {
		t.Errorf("Test case failed. Expected: %v, got: %v", nil, err)
	}

	assert.NotNil(t, conn, "Test case failed")
}

func Test_YCQL_InitializeYCQLFromConfigs(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	cfg := &config.MockConfig{
		Data: map[string]string{
			"PRE_CASS_DB_DIALECT":  "YCQL",
			"PRE_CASS_DB_PASS":     c.Get("YCQL_DB_PASS"),
			"PRE_CASS_DB_USER":     c.Get("YCQL_DB_USER"),
			"PRE_CASS_DB_PORT":     c.Get("YCQL_DB_PORT"),
			"PRE_CASS_DB_KEYSPACE": c.Get("CASS_DB_KEYSPACE"),
			"PRE_CASS_DB_TIMEOUT":  c.Get("CASS_DB_TIMEOUT"),
			"PRE_CASS_DB_HOST":     "localhost",
		},
	}

	conn, err := InitializeYCQLFromConfigs(cfg, logger, "PRE")
	if err != nil {
		t.Errorf("Test case failed. Expected: %v, got: %v", nil, err)
	}

	assert.NotNil(t, conn, "Test case failed")
}

func Test_YCQL_Configs(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	c := config.NewGoDotEnvProvider(logger, "../../configs")

	cfg := &config.MockConfig{
		Data: map[string]string{
			"CASS_DB_DIALECT":  "YCQL",
			"CASS_DB_PASS":     c.Get("YCQL_DB_PASS"),
			"CASS_DB_USER":     c.Get("YCQL_DB_USER"),
			"CASS_DB_PORT":     c.Get("YCQL_DB_PORT"),
			"CASS_DB_KEYSPACE": c.Get("CASS_DB_KEYSPACE"),
			"CASS_DB_TIMEOUT":  c.Get("CASS_DB_TIMEOUT"),
		},
	}

	testCases := []struct {
		host        string
		expectedStr string
	}{
		{"localhost", "Connected to YCQL"},
		{"invalidhost", "could not connect to YCQL"},
	}

	for i, tc := range testCases {
		b.Reset()

		k := &Gofr{}
		k.Logger = logger

		cfg.Data["CASS_DB_HOST"] = tc.host

		initializeCassandra(cfg, k)

		if !strings.Contains(b.String(), tc.expectedStr) {
			t.Errorf("FAILED case`%v`, expected: `%v` in the logs, got: %v", i, tc.expectedStr, b.String())
		}
	}
}
