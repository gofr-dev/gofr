package datastore

import (
	"bytes"
	"crypto/tls"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

func Test_CQL_Connection(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")

	testcases := []struct {
		hostName    string
		port        string
		user        string
		pass        string
		keySpace    string
		consistency string
		retryPolicy gocql.RetryPolicy
		dataCenter  string
	}{
		{
			c.Get("CASS_DB_HOST"), c.Get("CASS_DB_PORT"), c.Get("CASS_DB_USER"), c.Get("CASS_DB_PASS"), c.Get("CASS_DB_KEYSPACE"), localQuorum,
			&gocql.SimpleRetryPolicy{NumRetries: 3}, c.Get("DATA_CENTER"),
		},
		{
			c.Get("CASS_DB_HOST"), c.Get("CASS_DB_PORT"), c.Get("CASS_DB_USER"), c.Get("CASS_DB_PASS"), c.Get("CASS_DB_KEYSPACE"), "QUORUM",
			&gocql.SimpleRetryPolicy{NumRetries: 3}, c.Get("DATA_CENTER"),
		},
	}

	for k := range testcases {
		port, err := strconv.Atoi(testcases[k].port)
		if err != nil {
			port = 9042
		}

		cassandraConfig := CassandraCfg{
			Hosts:       testcases[k].hostName,
			Port:        port,
			Username:    testcases[k].user,
			Password:    testcases[k].pass,
			Keyspace:    testcases[k].keySpace,
			Consistency: testcases[k].consistency,
			RetryPolicy: testcases[k].retryPolicy,
			DataCenter:  testcases[k].dataCenter,
		}

		_, err = GetNewCassandra(logger, &cassandraConfig)
		if err != nil {
			t.Errorf("Connecting to cassandra-host: %v, cassandra-port: %v, user: %v, password: %v, keyspace: %v",
				cassandraConfig.Hosts,
				cassandraConfig.Port,
				cassandraConfig.Username,
				cassandraConfig.Password,
				cassandraConfig.Keyspace,
			)
			t.Error(err)
		}
	}
}

func Test_CQL_ImproperConnection(t *testing.T) {
	cassandraConfig := CassandraCfg{
		Hosts:       "fake host",
		Username:    "cassandra",
		Password:    "cassandra",
		Keyspace:    "system",
		Consistency: localQuorum,
		RetryPolicy: &gocql.SimpleRetryPolicy{NumRetries: 3},
	}
	logger := log.NewLogger()
	_, err := GetNewCassandra(logger, &cassandraConfig)

	if err == nil {
		t.Error(err)
	}
}

func Test_hostVerification(t *testing.T) {
	boolCheck := enableHostVerification("true")
	if !boolCheck {
		t.Errorf("Failed to enable host verification")
	}

	boolCheck = enableHostVerification("false")
	if boolCheck {
		t.Errorf("Failed to disable host verification")
	}
}

func Test_setTLSVersion(t *testing.T) {
	testCases := []struct {
		version         string
		expectedVersion uint16
	}{
		{
			"10", tls.VersionTLS10,
		},
		{
			"11", tls.VersionTLS11,
		},
		{
			"13", tls.VersionTLS13,
		},
		{
			"1234", tls.VersionTLS12,
		},
	}

	for i := range testCases {
		version := setTLSVersion(testCases[i].version)
		if version != testCases[i].expectedVersion {
			t.Errorf("TESTCASE %v, failed. Got %v, Expected %v", i+1, version, testCases[i].expectedVersion)
		}
	}
}

func Test_incorrectSSL_Connection(t *testing.T) {
	cassandraConfig := CassandraCfg{
		Hosts:       "fake host",
		Username:    "cassandra",
		Password:    "cassandra",
		Keyspace:    "system",
		Consistency: localQuorum,
		RetryPolicy: &gocql.SimpleRetryPolicy{NumRetries: 3},
	}
	logger := log.NewMockLogger(new(bytes.Buffer))
	_, err := GetNewCassandra(logger, &cassandraConfig)

	if err == nil {
		t.Error(err)
	}
}

func Test_CassandraQueryLog(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	c := config.NewGoDotEnvProvider(logger, "../../configs")

	port, err := strconv.Atoi(c.Get("CASS_DB_PORT"))
	if err != nil {
		port = 9042
	}

	conf := CassandraCfg{
		Hosts:       c.Get("CASS_DB_HOST"),
		Port:        port,
		Consistency: localQuorum,
		Username:    c.Get("CASS_DB_USER"),
		Password:    c.Get("CASS_DB_PASS"),
		Keyspace:    c.Get("CASS_DB_KEYSPACE"),
	}

	cassandra, err := GetNewCassandra(logger, &conf)
	if err != nil {
		t.Error(err)
	}

	{ // test query logs
		b.Reset()
		_ = cassandra.Session.Query("SELECT * FROM dummy_table").Exec()

		expectedLog := `"SELECT * FROM dummy_table"`

		if !strings.Contains(b.String(), expectedLog) {
			t.Errorf("[FAILED] expected: %v, got: %v", expectedLog, b.String())
		}

		if !strings.Contains(b.String(), `"level":"DEBUG"`) {
			t.Errorf("[FAILED] expected: %v, got: %v", expectedLog, b.String())
		}

		if !strings.Contains(b.String(), "cassandra") {
			t.Errorf("[FAILED] expected: %v, got: %v", "cassandra", b.String())
		}
	}

	{ // test batch execution logs
		b.Reset()

		batch := cassandra.Session.NewBatch(gocql.LoggedBatch)
		batch.Query("SELECT * FROM dummy_table")
		batch.Query("SELECT * FROM test_table")
		batch.Query("SELECT * FROM gofr_table")

		_ = cassandra.Session.ExecuteBatch(batch)

		expectedLog := `"SELECT * FROM dummy_table, SELECT * FROM test_table, SELECT * FROM gofr_table"`
		if !strings.Contains(b.String(), expectedLog) {
			t.Errorf("[FAILED] expected: %v, got: %v", expectedLog, b.String())
		}

		if !strings.Contains(b.String(), `"level":"DEBUG"`) {
			t.Errorf("[FAILED] expected: %v, got: %v", expectedLog, b.String())
		}

		if !strings.Contains(b.String(), "cassandra") {
			t.Errorf("[FAILED] expected: %v, got: %v", "cassandra", b.String())
		}
	}
}

func TestDataStore_CassandraHealthCheck(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	port, _ := strconv.Atoi(c.Get("CASS_DB_PORT"))
	testCases := []struct {
		c        CassandraCfg
		expected types.Health
	}{
		{CassandraCfg{Hosts: c.Get("CASS_DB_HOST"), Port: port, Username: c.Get("CASS_DB_USER"),
			Password: c.Get("CASS_DB_PASS"), Keyspace: c.Get("CASS_DB_KEYSPACE")},
			types.Health{Name: CassandraStore, Status: pkg.StatusUp, Host: c.Get("CASS_DB_HOST"), Database: "system"}},
		{CassandraCfg{Hosts: "random", Port: port, Username: c.Get("CASS_DB_USER"),
			Password: c.Get("CASS_DB_PASS"), Keyspace: c.Get("CASS_DB_KEYSPACE")},
			types.Health{Name: CassandraStore, Status: pkg.StatusDown, Host: "random", Database: "system"}},
	}

	for i, tc := range testCases {
		mockCassConfig := tc.c
		con, _ := GetNewCassandra(logger, &mockCassConfig)
		output := con.HealthCheck()

		assert.Equal(t, tc.expected, output, "TEST[%d], Failed.\n", i)
	}
}

// check log message returned from healthCheck
func Test_CassandraHealthCheck_Logger(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	con, _ := GetNewCassandra(logger, &CassandraCfg{})

	expectedResponse := types.Health{
		Name:   CassandraStore,
		Status: pkg.StatusDown,
	}

	expectedLogMessage := "Health check failed for cassandra Reason: Cassandra not initialized."

	output := con.HealthCheck()

	assert.Contains(t, b.String(), expectedLogMessage, "TESTCASE FAILED. \nexpected: %v, \ngot: %v", expectedLogMessage, b.String())

	assert.Equal(t, expectedResponse, output, "TESTCASE FAILED. \nexpected: %v, \ngot: %v", expectedResponse, output)
}

// Test_CassandraHealthCheck checks the health check when cassandra was connected but went down later
func Test_CassandraHealthCheck(t *testing.T) {
	logger := log.NewLogger()
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	port, _ := strconv.Atoi(c.Get("CASS_DB_PORT"))

	conf := &CassandraCfg{
		Hosts:    c.Get("CASS_DB_HOST"),
		Port:     port,
		Username: c.Get("CASS_DB_USER"),
		Password: c.Get("CASS_DB_PASS"),
		Keyspace: c.Get("CASS_DB_KEYSPACE"),
	}

	con, _ := GetNewCassandra(logger, conf)
	expected := types.Health{
		Name:     CassandraStore,
		Status:   pkg.StatusDown,
		Host:     conf.Hosts,
		Database: conf.Keyspace,
	}

	con.Session.Close()

	output := con.HealthCheck()

	assert.Equal(t, expected, output, "TEST Failed.\n")
}

func TestCQL_HealthCheck_NilObject(t *testing.T) {
	expected := types.Health{
		Name:   CassandraStore,
		Status: pkg.StatusDown,
	}

	var cql *Cassandra

	resp := cql.HealthCheck()

	assert.Equal(t, expected, resp, "TEST Failed.\n")
}

func Test_IncorrectSSLCertPathCql(t *testing.T) {
	logger := log.NewLogger()
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	port, _ := strconv.Atoi(c.Get("CASS_DB_PORT"))
	cfg := CassandraCfg{
		Hosts:               c.Get("CASS_DB_HOST"),
		Port:                port,
		Username:            c.Get("CASS_DB_USER"),
		Password:            c.Get("CASS_DB_PASS"),
		Keyspace:            c.Get("CASS_DB_KEYSPACE"),
		CertificateFile:     "private/node/certificate/path",
		KeyFile:             "private/node/key/path",
		RootCertificateFile: "root/certificate/file/path",
		HostVerification:    true,
		InsecureSkipVerify:  false,
	}

	_, err := GetNewCassandra(logger, &cfg)

	if err == nil {
		t.Error("Expected error \"unable to open CA certs\"")
	}
}
