package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	mysqlContainer "github.com/testcontainers/testcontainers-go/modules/mysql"
	redisContainer "github.com/testcontainers/testcontainers-go/modules/redis"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logging"
)

var (
	infra *testInfra
)

// testInfra holds the dependencies for the test suite
type testInfra struct {
	MySQLC     testcontainers.Container
	RedisC     testcontainers.Container
	DB         *sql.DB
	DBConnStr  string
	RedisAddr  string
	MockServer *httptest.Server
	BaseURL    string // Base URL of the running application
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	var err error
	infra = &testInfra{}

	// 1. Start MySQL
	infra.MySQLC, infra.DB, infra.DBConnStr, err = startMySQL(ctx)
	if err != nil {
		fmt.Printf("failed to start mysql: %v\n", err)
		os.Exit(1)
	}
	// 2. Start Redis
	infra.RedisC, infra.RedisAddr, err = startRedis(ctx)
	if err != nil {
		fmt.Printf("failed to start redis: %v\n", err)
		cleanup()
		os.Exit(1)
	}

	// 3. Start Mock Server for anotherService
	infra.MockServer = startMockServer()

	// 4. Setup Environment for the App (Global)
	httpPort := getFreePort()
	metricPort := getFreePort()
	setupGlobalEnv(httpPort, metricPort)
	infra.BaseURL = fmt.Sprintf("http://localhost:%d", httpPort)

	// 5. Start the Application Once
	go main()

	// 6. Wait for App Health
	if err := waitForAppStart(httpPort); err != nil {
		fmt.Printf("app failed to start: %v\n", err)
		cleanup()
		os.Exit(1)
	}

	// 7. Run Tests
	code := m.Run()

	// 8. Teardown
	cleanup()
	os.Exit(code)
}

func cleanup() {
	ctx := context.Background()
	if infra.MySQLC != nil {
		infra.MySQLC.Terminate(ctx)
	}
	if infra.RedisC != nil {
		infra.RedisC.Terminate(ctx)
	}
	if infra.MockServer != nil {
		infra.MockServer.Close()
	}
	if infra.DB != nil {
		infra.DB.Close()
	}
}

func setupGlobalEnv(httpPort, metricPort int) {
	os.Setenv("GOFR_TELEMETRY", "false")
	os.Setenv("APP_NAME", "http-server-test")
	os.Setenv("GOFR_ENV", "test")
	os.Setenv("GOFR_CONFIG_PATH", "./non-existent-dir")
	os.Setenv("LOG_LEVEL", "debug")

	// Parse DB Config
	cfg, err := mysql.ParseDSN(infra.DBConnStr)
	if err != nil {
		panic(err)
	}
	host, port, _ := strings.Cut(cfg.Addr, ":")
	if host == "localhost" {
		host = "127.0.0.1"
	}

	// DB
	os.Setenv("DB_HOST", host)
	os.Setenv("DB_PORT", port)
	os.Setenv("DB_USER", cfg.User)
	os.Setenv("DB_PASSWORD", cfg.Passwd)
	os.Setenv("DB_NAME", cfg.DBName)
	os.Setenv("DB_DIAL_TIMEOUT", "60s")

	// Redis
	rHost, rPort, _ := strings.Cut(infra.RedisAddr, ":")
	if rHost == "localhost" {
		rHost = "127.0.0.1"
	}
	os.Setenv("REDIS_HOST", rHost)
	os.Setenv("REDIS_PORT", rPort)

	// App Ports
	os.Setenv("HTTP_PORT", strconv.Itoa(httpPort))
	os.Setenv("METRICS_PORT", strconv.Itoa(metricPort))

	// Mock Service URL
	os.Setenv("ANOTHERSERVICE", infra.MockServer.URL)
}

func waitForAppStart(port int) error {
	host := fmt.Sprintf("http://localhost:%d", port)
	client := &http.Client{Timeout: 2 * time.Second}

	for i := 0; i < 60; i++ { // Wait up to 60 seconds
		resp, err := client.Get(host + "/.well-known/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("timeout waiting for health check on port %d", port)
}

func getFreePort() int {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func startMySQL(ctx context.Context) (testcontainers.Container, *sql.DB, string, error) {
	mysqlC, err := mysqlContainer.Run(ctx,
		"mysql:8",
		mysqlContainer.WithDatabase("testdb"),
		mysqlContainer.WithUsername("root"),
		mysqlContainer.WithPassword("password"),
	)
	if err != nil {
		return nil, nil, "", err
	}

	connStr, err := mysqlC.ConnectionString(ctx, "tls=false")
	if err != nil {
		return mysqlC, nil, "", err
	}

	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return mysqlC, nil, connStr, err
	}

	for i := 0; i < 30; i++ {
		err = db.Ping()
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		return mysqlC, db, connStr, fmt.Errorf("mysql failed to become ready: %w", err)
	}

	return mysqlC, db, connStr, nil
}

func startRedis(ctx context.Context) (testcontainers.Container, string, error) {
	redisC, err := redisContainer.Run(ctx, "redis:7")
	if err != nil {
		return nil, "", err
	}

	endpoint, err := redisC.Endpoint(ctx, "")
	if err != nil {
		return redisC, "", err
	}

	return redisC, endpoint, nil
}

func startMockServer() *httptest.Server {
	mockMux := http.NewServeMux()
	mockMux.HandleFunc("/redis", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":"mock data"}`))
	})
	return httptest.NewServer(mockMux)
}

func Test_Integration(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}

	t.Run("HelloHandler", func(t *testing.T) {
		resp, body, err := makeRequest(client, http.MethodGet, infra.BaseURL+"/hello", nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, string(body), "Hello World!")
	})

	t.Run("HelloHandler_WithName", func(t *testing.T) {
		resp, body, err := makeRequest(client, http.MethodGet, infra.BaseURL+"/hello?name=GoFr", nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, string(body), "Hello GoFr!")
	})

	t.Run("RedisHandler", func(t *testing.T) {
		resp, body, err := makeRequest(client, http.MethodGet, infra.BaseURL+"/redis", nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, string(body), "test-value")
	})

	t.Run("MysqlHandler", func(t *testing.T) {
		resp, body, err := makeRequest(client, http.MethodGet, infra.BaseURL+"/mysql", nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		// select 2+2 returns 4
		assert.Contains(t, string(body), "4")
	})

	t.Run("TraceHandler", func(t *testing.T) {
		resp, body, err := makeRequest(client, http.MethodGet, infra.BaseURL+"/trace", nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, string(body), "mock data")
	})

	t.Run("ErrorHandler", func(t *testing.T) {
		resp, body, err := makeRequest(client, http.MethodGet, infra.BaseURL+"/error", nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		assert.Contains(t, string(body), "some error occurred")
	})
}

func makeRequest(client *http.Client, method, url string, body interface{}) (*http.Response, []byte, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		bodyReader = strings.NewReader(string(jsonBody))
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	return resp, respBody, err
}

// Unit tests using mocks still preserved if needed, but the user asked to change the file to use containers.
// I'll keep the unit tests that don't conflict.

func createTestContext(method, url string, mockContainer *container.Container) *gofr.Context {
	req := httptest.NewRequest(method, url, nil)
	req.Header.Set("Content-Type", "application/json")
	gofrReq := gofrHTTP.NewRequest(req)

	var c *container.Container
	if mockContainer != nil {
		c = mockContainer
	} else {
		c = &container.Container{Logger: logging.NewLogger(logging.DEBUG)}
	}

	return &gofr.Context{
		Context:       req.Context(),
		Request:       gofrReq,
		Container:     c,
		ContextLogger: *logging.NewContextLogger(req.Context(), c.Logger),
	}
}

func TestHelloHandler_Unit(t *testing.T) {
	ctx := createTestContext(http.MethodGet, "/hello?name=test", nil)
	resp, err := HelloHandler(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "Hello test!", resp)
}

func TestMysqlHandler_Unit(t *testing.T) {
	mockContainer, mocks := container.NewMockContainer(t)
	mocks.SQL.ExpectQuery("select 2+2").
		WillReturnRows(mocks.SQL.NewRows([]string{"value"}).AddRow(4))

	ctx := createTestContext(http.MethodGet, "/mysql", mockContainer)
	resp, err := MysqlHandler(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 4, resp)
}
