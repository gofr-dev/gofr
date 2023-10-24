package datastore

import (
	"bytes"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/gocql"

	"gofr.dev/pkg"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

func TestQueryLogger_String(t *testing.T) {
	logger := &QueryLogger{}

	result := logger.String()

	expected := `{"query":null,"duration":0,"datastore":""}`

	if result != expected {
		t.Errorf("Unexpected result. Expected: %s, Got: %s", expected, result)
	}
}

func Test_NewYCQL_Connection(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")

	testcases := []struct {
		hostNames   string
		port        string
		user        string
		pass        string
		keySpace    string
		consistency string
		Timeout     int
		RetryPolicy int
		dataCenter  string
	}{
		{
			c.Get("CASS_DB_HOST"), c.Get("YCQL_DB_PORT"), c.Get("YCQL_DB_USER"), c.Get("YCQL_DB_PASS"),
			c.Get("CASS_DB_KEYSPACE"), localQuorum, 600, 5, c.Get("DATA_CENTER"),
		},
		{
			c.Get("CASS_DB_HOST"), c.Get("YCQL_DB_PORT"), c.Get("YCQL_DB_USER"), c.Get("YCQL_DB_PASS"),
			c.Get("CASS_DB_KEYSPACE"), "QUORUM", 600, 5, c.Get("DATA_CENTER"),
		},
	}

	for k := range testcases {
		port, err := strconv.Atoi(testcases[k].port)
		if err != nil {
			port = 9042
		}

		yugabyteDBConfig := CassandraCfg{
			Hosts:       testcases[k].hostNames,
			Port:        port,
			Username:    testcases[k].user,
			Password:    testcases[k].pass,
			Keyspace:    testcases[k].keySpace,
			Consistency: testcases[k].consistency,
			Timeout:     testcases[k].Timeout,
			DataCenter:  testcases[k].dataCenter,
		}

		conn, err := GetNewYCQL(logger, &yugabyteDBConfig)
		conn.Cluster.RetryPolicy = &gocql.SimpleRetryPolicy{NumRetries: testcases[k].RetryPolicy}

		if err != nil {
			t.Errorf("Connecting to yugabyteDB-host: %v, Cassandra-port: %v, user: %v, password: %v,"+
				" keyspace: %v, consistency: %v, timeout: %v, datacenter: %v",
				yugabyteDBConfig.Hosts,
				yugabyteDBConfig.Port,
				yugabyteDBConfig.Username,
				yugabyteDBConfig.Password,
				yugabyteDBConfig.Keyspace,
				yugabyteDBConfig.Consistency,
				yugabyteDBConfig.Timeout,
				yugabyteDBConfig.DataCenter,
			)
			t.Error(err)
		} else {
			conn.Session.Close()
		}
	}
}

func Test_NewYCQL_ImproperConnection(t *testing.T) {
	yugabyteDBConfig := CassandraCfg{
		Hosts:       "fake host",
		Username:    "Cassandra",
		Password:    "Cassandra",
		Keyspace:    "system",
		Consistency: localQuorum,
		Timeout:     600,
	}

	logger := log.NewMockLogger(io.Discard)
	_, err := GetNewYCQL(logger, &yugabyteDBConfig)

	if err == nil {
		t.Error(err)
	}
}

func Test_YCQLQueryLog(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	port, _ := strconv.Atoi(c.Get("YCQL_DB_PORT"))

	ycqlDB, err := GetNewYCQL(logger, &CassandraCfg{
		Hosts:       "localhost",
		Port:        port,
		Consistency: "LOCAL_QUORUM",
		Username:    c.Get("YCQL_DB_USER"),
		Password:    c.Get("YCQL_DB_PASS"),
		Keyspace:    "system",
	})
	if err != nil {
		t.Error(err)
	}

	{ // test query logs
		b.Reset()
		_ = ycqlDB.Session.Query("SELECT * FROM dummy_table").Exec()

		expectedLog := `"SELECT * FROM dummy_table`

		if !strings.Contains(b.String(), expectedLog) {
			t.Errorf("[FAILED] expected: %v, got: %v", expectedLog, b.String())
		}

		if !strings.Contains(b.String(), `"level":"DEBUG"`) {
			t.Errorf("[FAILED] expected: %v, got: %v", expectedLog, b.String())
		}

		if !strings.Contains(b.String(), "ycql") {
			t.Errorf("[FAILED] expected: %v, got: %v", "YCQL", b.String())
		}
	}

	{ // test batch execution logs
		b.Reset()

		batch := ycqlDB.Session.NewBatch(gocql.LoggedBatch)
		batch.Query("SELECT * FROM dummy_table")
		batch.Query("SELECT * FROM test_table")
		batch.Query("SELECT * FROM gofr_table")

		_ = ycqlDB.Session.ExecuteBatch(batch)
		expectedLog := `"SELECT * FROM dummy_table, SELECT * FROM test_table, SELECT * FROM gofr_table"`
		if !strings.Contains(b.String(), expectedLog) {
			t.Errorf("[FAILED] expected: %v, got: %v", expectedLog, b.String())
		}

		if !strings.Contains(b.String(), "ycql") {
			t.Errorf("[FAILED] expected: %v, got: %v", "YCQL ", b.String())

			if !strings.Contains(b.String(), `"level":"DEBUG"`) {
				t.Errorf("[FAILED] expected: %v, got: %v", expectedLog, b.String())
			}
		}
	}
}

func TestDataStore_YCQLHealthCheck(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	port, _ := strconv.Atoi(c.Get("CASS_DB_PORT"))
	testCases := []struct {
		c        CassandraCfg
		expected types.Health
	}{
		{
			CassandraCfg{
				Hosts:       c.Get("CASS_DB_HOST"),
				Port:        port,
				Consistency: localQuorum,
				Username:    c.Get("CASS_DB_USER"),
				Password:    c.Get("CASS_DB_PASS"),
				Keyspace:    c.Get("CASS_DB_KEYSPACE"),
			},
			types.Health{
				Name:     "ycql",
				Status:   "UP",
				Host:     c.Get("CASS_DB_HOST"),
				Database: "system",
			},
		},
		{
			CassandraCfg{
				Hosts:    "random",
				Port:     port,
				Username: c.Get("YCQL_DB_USER"),
				Password: c.Get("YCQL_DB_PASS"),
				Keyspace: c.Get("CASS_DB_KEYSPACE"),
			},
			types.Health{
				Name:     Ycql,
				Status:   pkg.StatusDown,
				Host:     "random",
				Database: "system",
			},
		},
	}

	for i, tc := range testCases {
		con, _ := GetNewYCQL(logger, &tc.c)
		output := con.HealthCheck()

		if !reflect.DeepEqual(tc.expected, output) {
			t.Errorf("[FAILED]%v expected: %v, got: %v", i, tc.expected, output)
		}
	}
}

func Test_YCQLHealthCheck_Logs(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	con, _ := GetNewYCQL(logger, &CassandraCfg{})

	expectedResponse := types.Health{
		Name:     Ycql,
		Status:   pkg.StatusDown,
		Host:     "",
		Database: "",
	}

	expectedLogMessage := "Health check failed for ycql Reason: YCQL not initialized."

	output := con.HealthCheck()

	assert.Contains(t, b.String(), expectedLogMessage, "TESTCASE FAILED. \nexpected: %v, \ngot: %v", expectedLogMessage, b.String())

	assert.Equal(t, expectedResponse, output, "TESTCASE FAILED. \nexpected: %v, \ngot: %v", expectedResponse, output)
}

// Test_YCQLHealthCheck checks the health check when ycql was connected but went down later
func Test_YCQLHealthCheck_Down(t *testing.T) {
	logger := log.NewLogger()
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	port, _ := strconv.Atoi(c.Get("YCQL_DB_PORT"))

	con, _ := GetNewYCQL(logger, &CassandraCfg{
		Hosts:    c.Get("CASS_DB_HOST"),
		Port:     port,
		Username: c.Get("YCQL_DB_USER"),
		Password: c.Get("YCQL_DB_PASS"),
		Keyspace: c.Get("CASS_DB_KEYSPACE"),
	})

	expected := types.Health{
		Name:     Ycql,
		Status:   pkg.StatusDown,
		Host:     c.Get("CASS_DB_HOST"),
		Database: c.Get("CASS_DB_KEYSPACE"),
	}

	con.Session.Close()

	output := con.HealthCheck()

	if !reflect.DeepEqual(expected, output) {
		t.Errorf("expected: %v, got: %v", expected, output)
	}
}

func TestYCQL_HealthCheck_NilObject(t *testing.T) {
	expected := types.Health{
		Name:   Ycql,
		Status: pkg.StatusDown,
	}

	var ycql *YCQL

	resp := ycql.HealthCheck()
	if !reflect.DeepEqual(expected, resp) {
		t.Errorf("expected: %v, got: %v", expected, resp)
	}
}

func Test_IncorrectSSLCertPathYCQL(t *testing.T) {
	logger := log.NewLogger()
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	port, _ := strconv.Atoi(c.Get("YCQL_DB_PORT"))
	cfg := CassandraCfg{
		Hosts:               c.Get("CASS_DB_HOST"),
		Keyspace:            c.Get("CASS_DB_KEYSPACE"),
		CertificateFile:     "private/node/certificate/path",
		KeyFile:             "private/node/key/path",
		RootCertificateFile: "root/certificate/file/path",
		HostVerification:    true,
		InsecureSkipVerify:  false,
	}
	cfg.Port = port
	cfg.Username = c.Get("YCQL_DB_USER")
	cfg.Password = c.Get("YCQL_DB_PASS")

	_, err := GetNewYCQL(logger, &cfg)

	if err == nil {
		t.Error("Expected error \"unable to open CA certs\"")
	}
}
