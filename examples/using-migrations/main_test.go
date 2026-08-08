package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

const (
	configFolder = "./configs"

	// testDatabase is created and dropped by the test run. The example's own configs point at the
	// shared `test` database, which every other example migrates into as well - migration
	// bookkeeping there is keyed by version only, so the highest version any example has run wins
	// and the others get skipped. Migrating into a database this test owns keeps the run
	// independent of the other examples and of what previous runs left behind.
	testDatabase = "test_using_migrations"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")

	c := config.NewEnvFile(configFolder, logging.NewLogger(logging.ERROR))

	if err := setupSQL(c); err != nil {
		fmt.Fprintf(os.Stderr, "could not set up the test database: %v\n", err)
		os.Exit(1)
	}

	// Redis has no per-test namespace here, so clear the keys this example writes instead. Left
	// behind, the migration bookkeeping makes the runner skip the migrations on the next run.
	if err := resetRedis(c); err != nil {
		fmt.Fprintf(os.Stderr, "could not reset redis: %v\n", err)
		os.Exit(1)
	}

	m.Run()
}

// setupSQL recreates the database this test migrates into and points the example at it.
func setupSQL(c config.Config) error {
	// Connecting without a database, as the one this test uses is about to be created.
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/", c.Get("DB_USER"), c.Get("DB_PASSWORD"),
		net.JoinHostPort(c.Get("DB_HOST"), c.Get("DB_PORT")))

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}

	defer db.Close()

	for _, query := range []string{"DROP DATABASE IF EXISTS " + testDatabase, "CREATE DATABASE " + testDatabase} {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}

	// The config files are read again by gofr.New(), which does not override what is already set.
	return os.Setenv("DB_NAME", testDatabase)
}

func resetRedis(c config.Config) error {
	client := redis.NewClient(&redis.Options{Addr: net.JoinHostPort(c.Get("REDIS_HOST"), c.Get("REDIS_PORT"))})

	defer client.Close()

	// gofr_migrations holds the bookkeeping, gofr_migrations_lock the migration lock, and
	// Umang is the key the Redis migration writes.
	return client.Del(context.Background(), "gofr_migrations", "gofr_migrations_lock", "Umang").Err()
}

func TestExampleMigration(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	go main()
	// The migrations run at startup, which a fixed sleep would not reliably cover.
	testutil.WaitForHTTPServer(t, configs.HTTPHost)

	tests := []struct {
		desc       string
		method     string
		path       string
		body       []byte
		statusCode int
	}{
		{"post new employee with valid data", http.MethodPost, "/employee",
			[]byte(`{"id":2,"name":"John","gender":"Male","contact_number":1234567890,"dob":"2000-01-01"}`), 201},
		{"get employee with valid name", http.MethodGet, "/employee?name=John", nil, 200},
		{"get employee does not exist", http.MethodGet, "/employee?name=Invalid", nil, 500},
		{"get employee with empty name", http.MethodGet, "/employee", nil, http.StatusInternalServerError},
		{"post new employee with invalid data", http.MethodPost, "/employee", []byte(`{"id":2"}`),
			http.StatusInternalServerError},
		{"post new employee with invalid gender", http.MethodPost, "/employee",
			[]byte(`{"id":2,"name":"John","gender":"Male123","contact_number":1234567890,"dob":"2000-01-01"}`), 500},
	}

	for i, tc := range tests {
		req, _ := http.NewRequest(tc.method, configs.HTTPHost+tc.path, bytes.NewBuffer(tc.body))
		req.Header.Set("content-type", "application/json")
		c := http.Client{}
		resp, err := c.Do(req)

		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.statusCode, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
