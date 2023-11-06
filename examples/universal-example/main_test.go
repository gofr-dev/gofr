//go:build !integration

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"gofr.dev/examples/universal-example/avro/handlers"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/request"
	"gofr.dev/pkg/log"
)

func TestMain(m *testing.M) {
	app := gofr.New()

	cassandraTableInitialization(app)

	postgresTableInitialization(app)

	// avro schema registry test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		re := map[string]interface{}{
			"subject": "employee-value",
			"version": 3,
			"id":      303,
			"schema": "{\"type\":\"record\",\"name\":\"employee\"," +
				"\"fields\":[{\"name\":\"Id\",\"type\":\"string\"}," +
				"{\"name\":\"Name\",\"type\":\"string\"}," +
				"{\"name\":\"Phone\",\"type\":\"string\"}," +
				"{\"name\":\"Email\",\"type\":\"string\"}," +
				"{\"name\":\"City\",\"type\":\"string\"}]}",
		}

		reBytes, _ := json.Marshal(re)
		w.Header().Set("Content-type", "application/json")
		_, _ = w.Write(reBytes)
	}))

	schemaURL := os.Getenv("AVRO_SCHEMA_URL")
	os.Setenv("AVRO_SCHEMA_URL", ts.URL)

	topic := os.Getenv("KAFKA_TOPIC")
	os.Setenv("KAFKA_TOPIC", "avro-pubsub")

	defer func() {
		os.Setenv("AVRO_SCHEMA_URL", schemaURL)
		os.Setenv("KAFKA_TOPIC", topic)
	}()

	//nolint:gocritic //os.Exit will exit, and `defer func(){...}(...)`
	os.Exit(m.Run())
}

func TestUniversalIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	// call the main function
	go main()
	// sleep, so that every data stores get initialized properly
	time.Sleep(5 * time.Second)

	testDataStores(t)
	testKafkaDataStore(t)
	testEventhub(t)
}

func testDataStores(tb testing.TB) {
	tests := []struct {
		testID             int
		method             string
		endpoint           string
		expectedStatusCode int
		body               []byte
	}{
		// Cassandra
		{1, http.MethodGet, "/cassandra/employee?name=Aman", http.StatusOK, nil},
		{2, http.MethodPost, "/cassandra/employee", http.StatusCreated,
			[]byte(`{"id": 5, "name": "Sukanya", "phone": "01477", "email":"sukanya@gofr.dev", "city":"Guwahati"}`)},
		{3, http.MethodGet, "/cassandra/unknown", http.StatusNotFound, nil},
		// Redis
		{4, http.MethodGet, "/redis/config/key123", http.StatusInternalServerError, nil},
		{5, http.MethodPost, "/redis/config", http.StatusCreated, []byte(`{}`)},
		// Postgres
		{6, http.MethodGet, "/pgsql/employee", http.StatusOK, nil},
		{7, http.MethodPost, "/pgsql/employee", http.StatusCreated,
			[]byte(`{"id": 5, "name": "Sukanya", "phone": "01477", "email":"sukanya@gofr.dev", "city":"Guwahati"}`)},
	}
	for _, tc := range tests {
		req, _ := request.NewMock(tc.method, "http://localhost:9095"+tc.endpoint, bytes.NewBuffer(tc.body))
		client := http.Client{}

		resp, err := client.Do(req)
		if err != nil {
			tb.Errorf("TestCase[%v] \t FAILED \nGot Error: %v", tc.testID, err)
			continue
		}

		if resp != nil && resp.StatusCode != tc.expectedStatusCode {
			tb.Errorf("Testcase[%v] Failed.\tExpected %v\tGot %v\n", tc.testID, tc.expectedStatusCode, resp.StatusCode)
		}

		if resp != nil {
			resp.Body.Close()
		}
	}
}

//nolint:gocognit // can't break the function because of retry logic
func testKafkaDataStore(tb testing.TB) {
	tests := []struct {
		testID             int
		method             string
		endpoint           string
		expectedResponse   string
		expectedStatusCode int
	}{
		{8, http.MethodGet, "http://localhost:9095/avro/pub?id=1", "", http.StatusOK},
		{9, http.MethodGet, "http://localhost:9095/avro/sub", "1", http.StatusOK},
	}

	for _, tc := range tests {
		req, _ := request.NewMock(tc.method, tc.endpoint, nil)
		c := http.Client{}

		for i := 0; i < 5; i++ {
			resp, _ := c.Do(req)

			if resp != nil && resp.StatusCode != tc.expectedStatusCode {
				// retry is required since, creation of topic takes time
				if checkRetry(resp.Body) {
					time.Sleep(3 * time.Second)
					continue
				}

				tb.Errorf("Test %v: Failed.\tExpected %v\tGot %v\n", tc.testID, tc.expectedStatusCode, resp.StatusCode)

				return
			}

			// checks whether bind avro.Unmarshal functionality works fine
			if tc.expectedResponse != "" && resp.Body != nil {
				body, _ := io.ReadAll(resp.Body)

				m := struct {
					Data handlers.Employee `json:"data"`
				}{}
				_ = json.Unmarshal(body, &m)

				if m.Data.ID != tc.expectedResponse {
					tb.Errorf("Expected: %v, Got: %v", tc.expectedResponse, m.Data.ID)
				}
			}

			if resp != nil {
				resp.Body.Close()
			}

			break
		}
	}
}

//nolint:gocognit // braking down the function will reduce the readability
func testEventhub(tb testing.TB) {
	tests := []struct {
		testID             int
		method             string
		endpoint           string
		expectedResponse   string
		expectedStatusCode int
	}{
		{10, http.MethodGet, "http://localhost:9095/eventhub/pub?id=1", "", http.StatusOK},
		{11, http.MethodGet, "http://localhost:9095/eventhub/sub", "1", http.StatusOK},
	}

	for _, tc := range tests {
		req, _ := request.NewMock(tc.method, tc.endpoint, nil)
		c := http.Client{}
		resp, _ := c.Do(req)

		if resp != nil && resp.StatusCode != tc.expectedStatusCode {
			// required because eventhub is shared and there can be messages with avro or without avro
			// messages without avro would return 200 as we do json.Marshal to a map
			// messages with avro would return 206 as it would have to go through avro.Marshal
			// we can't use any avro schema as any schema can be used
			if resp.StatusCode != http.StatusPartialContent {
				tb.Errorf("Test %v: Failed.\tExpected %v\tGot %v\n", tc.testID, tc.expectedStatusCode, resp.StatusCode)
			}
		}

		if resp != nil {
			resp.Body.Close()
		}
	}
}

// Cassandra Table initialization, Remove table if already exists
func cassandraTableInitialization(app *gofr.Gofr) {
	logger := log.NewLogger()
	c := config.NewGoDotEnvProvider(logger, "configs")

	// Keyspace Creation for cassandra
	cassandraPort, _ := strconv.Atoi(c.Get("CASS_DB_PORT"))
	cassandraCfg := datastore.CassandraCfg{
		Hosts:    c.Get("CASS_DB_HOST"),
		Port:     cassandraPort,
		Username: c.Get("CASS_DB_USER"),
		Password: c.Get("CASS_DB_PASS"),
		Keyspace: "system",
	}

	cassDB, err := datastore.GetNewCassandra(logger, &cassandraCfg)
	if err == nil {
		err = cassDB.Session.Query("CREATE KEYSPACE test WITH replication = {'class':'SimpleStrategy', 'replication_factor' : 1};").Exec()
	}

	for i := 0; err != nil && i < 10; i++ {
		time.Sleep(5 * time.Second)

		cassDB, err = datastore.GetNewCassandra(logger, &cassandraCfg)
		if err != nil {
			continue
		}

		err = cassDB.Session.Query(
			"CREATE KEYSPACE IF NOT EXISTS test WITH replication = {'class':'SimpleStrategy', 'replication_factor' : 1};").Exec()
	}

	queryStr := "DROP TABLE IF EXISTS employees"
	if e := app.Cassandra.Session.Query(queryStr).Exec(); e != nil {
		app.Logger.Errorf("Got error while dropping the existing table employees: ", e)
	}

	queryStr = "CREATE TABLE IF NOT EXISTS employees (id int, name text, phone text, email text, city text, PRIMARY KEY (id) )"

	err = app.Cassandra.Session.Query(queryStr).Exec()
	if err != nil {
		app.Logger.Errorf("Failed creation of Table employees :%v", err)
	} else {
		app.Logger.Info("Table employees created Successfully")
	}
}

// Postgres Table initialization, Remove table if already exists
func postgresTableInitialization(g *gofr.Gofr) {
	if g.DB() == nil {
		return
	}

	query := `DROP TABLE IF EXISTS employees`
	if _, err := g.DB().Exec(query); err != nil {
		g.Logger.Errorf("Got error while dropping the existing table employees: ", err)
	}

	queryTable := `
 	   CREATE TABLE IF NOT EXISTS employees (
	   id         int primary key,
	   name       varchar (50),
 	   phone      varchar(50),
 	   email      varchar(50) ,
 	   city       varchar(50))
	`

	if _, err := g.DB().Exec(queryTable); err != nil {
		g.Logger.Errorf("Got error while sourcing the schema: ", err)
	}
}

func checkRetry(respBody io.Reader) bool {
	body, _ := io.ReadAll(respBody)

	errResp := struct {
		Errors []errors.Response `json:"errors"`
	}{}

	if len(errResp.Errors) == 0 {
		return false
	}

	_ = json.Unmarshal(body, &errResp)

	return strings.Contains(errResp.Errors[0].Reason, "Leader Not Available: the cluster is in the middle of a leadership election")
}
