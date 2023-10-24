package main

import (
	"bytes"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/request"
	"gofr.dev/pkg/log"
)

func TestMain(m *testing.M) {
	logger := log.NewLogger()
	c := config.NewGoDotEnvProvider(logger, "./configs")
	port, _ := strconv.Atoi(c.Get("CASS_DB_PORT"))
	ycqlCfg := datastore.CassandraCfg{
		Hosts:    c.Get("CASS_DB_HOST"),
		Port:     port,
		Username: c.Get("CASS_DB_USER"),
		Password: c.Get("CASS_DB_PASS"),
		Keyspace: "system",
	}

	ycqlDB, err := datastore.GetNewYCQL(logger, &ycqlCfg)
	if err != nil {
		logger.Errorf("Failed, unable to connect to ycql")
	}

	err = ycqlDB.Session.Query(
		"CREATE KEYSPACE IF NOT EXISTS test WITH REPLICATION = {'class': 'SimpleStrategy', 'replication_factor': '1'} " +
			"AND DURABLE_WRITES = true;").Exec()
	if err != nil {
		logger.Errorf("Failed to create keyspace :%v", err)
	}

	ycqlCfg.Keyspace = "test"

	ycqlDB, err = datastore.GetNewYCQL(logger, &ycqlCfg)
	if err != nil {
		logger.Errorf("Failed to connect with ycql :%v", err)
	}

	// remove table if exist
	_ = ycqlDB.Session.Query("DROP TABLE IF EXISTS shop").Exec()

	queryStr := "CREATE TABLE shop (id int PRIMARY KEY, name varchar, location varchar , state varchar ) " +
		"WITH transactions = { 'enabled' : true };"

	err = ycqlDB.Session.Query(queryStr).Exec()
	if err != nil {
		logger.Errorf("Failed creation of Table shop :%v", err)
	} else {
		logger.Info("Table shop created Successfully")
	}

	os.Exit(m.Run())
}

func TestIntegrationShop(t *testing.T) {
	// call  the main function
	go main()

	time.Sleep(time.Second * 5)

	testcases := []struct {
		desc       string
		method     string
		endpoint   string
		statusCode int
		body       []byte
	}{
		{"get with name", http.MethodGet, "shop?name=Vikash", http.StatusOK, nil},
		{"create by id 4", http.MethodPost, "shop", http.StatusCreated,
			[]byte(`{"id": 4, "name": "Puma", "location": "Belandur" , "state": "karnataka"}`)},
		{"create by id 7", http.MethodPost, "shop", http.StatusCreated,
			[]byte(`{"id": 7, "name": "Kalash", "location": "Jehanabad", "state": "Bihar"}`)},
		{"get at invalid endpoint", http.MethodGet, "unknown", http.StatusNotFound, nil},
		{"get shop by id at invalid endpoint", http.MethodGet, "shop/id", http.StatusNotFound, nil},
		{"update shop at invalid endpoint", http.MethodPut, "shop", http.StatusMethodNotAllowed, nil},
		{"delete shop", http.MethodDelete, "shop/4", http.StatusNoContent, nil},
	}
	for i, tc := range testcases {
		req, _ := request.NewMock(tc.method, "http://localhost:8085/"+tc.endpoint, bytes.NewBuffer(tc.body))

		cl := http.Client{}

		resp, err := cl.Do(req)
		if err != nil {
			t.Errorf("TEST[%v] Failed.\tHTTP request encountered Err: %v", i, err)

			continue // move to next test
		}

		if resp.StatusCode != tc.statusCode {
			t.Errorf("TEST[%v] Failed.\tExpected %v\tGot %v\n%s", i, tc.statusCode, resp.StatusCode, tc.desc)
		}

		_ = resp.Body.Close()
	}
}
