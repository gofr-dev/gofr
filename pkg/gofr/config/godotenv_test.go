package config

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/logging"
)

func clearAllEnv() {
	for _, envVar := range os.Environ() {
		key, _, _ := strings.Cut(envVar, "=")
		_ = os.Unsetenv(key)
	}
}

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	os.Exit(m.Run())
}

func Test_EnvSuccess(t *testing.T) {
	clearAllEnv()

	envData := map[string]string{
		"DB_URL":     "localhost:5432",
		"API_KEY":    "your_api_key_here",
		"small_case": "small_case_value",
	}

	logger := logging.NewMockLogger(logging.DEBUG)

	dir := t.TempDir()

	// Call the function to create the .env file
	createEnvFile(t, dir, ".env", envData)

	env := NewEnvFile(dir, logger)

	assert.Equal(t, "localhost:5432", env.Get("DB_URL"), "TEST Failed.\n godotenv success")
	assert.Equal(t, "your_api_key_here", env.GetOrDefault("API_KEY", "xyz"), "TEST Failed.\n godotenv success")
	assert.Equal(t, "test", env.GetOrDefault("DATABASE", "test"), "TEST Failed.\n godotenv success")
	assert.Equal(t, "small_case_value", env.Get("small_case"), "TEST Failed.\n godotenv success")
}

func Test_EnvSuccess_AppEnv_Override(t *testing.T) {
	t.Setenv("APP_ENV", "prod")

	envData := map[string]string{
		"DB_URL": "localhost:5432",
	}

	dir := t.TempDir()

	// Call the function to create the .env file
	createEnvFile(t, dir, ".env", envData)

	// override database url in '.prod.env' file to test if value if being overridden
	createEnvFile(t, dir, ".prod.env", map[string]string{"DB_URL": "localhost:2001"})

	logger := logging.NewMockLogger(logging.DEBUG)

	env := NewEnvFile(dir, logger)

	assert.Equal(t, "localhost:2001", env.Get("DB_URL"), "TEST Failed.\n godotenv success")
}

func Test_EnvSuccess_Local_Override(t *testing.T) {
	clearAllEnv()

	envData := map[string]string{
		"API_KEY": "your_api_key_here",
	}

	dir := t.TempDir()

	// Call the function to create the .env file
	createEnvFile(t, dir, ".env", envData)

	// override database url in '.prod.env' file to test if value if being overridden
	createEnvFile(t, dir, ".local.env", map[string]string{"API_KEY": "overloaded_api_key"})

	logger := logging.NewMockLogger(logging.DEBUG)

	env := NewEnvFile(dir, logger)

	assert.Equal(t, "overloaded_api_key", env.Get("API_KEY"), "TEST Failed.\n godotenv success")
}

func Test_EnvSuccess_SystemEnv_Override(t *testing.T) {
	clearAllEnv()

	// Set initial environment variables
	envData := map[string]string{
		"TEST_ENV": "env",
	}

	dir := t.TempDir()

	// Create the .env file
	createEnvFile(t, dir, ".env", envData)

	// Create the override file
	createEnvFile(t, dir, ".local.env", map[string]string{"TEST_ENV": "local"})

	t.Setenv("TEST_ENV", "system")

	logger := logging.NewMockLogger(logging.DEBUG)

	env := NewEnvFile(dir, logger)

	assert.Equal(t, "system", env.Get("TEST_ENV"), "TEST Failed.\n system env override")
}

func Test_EnvFailureWithHyphen(t *testing.T) {
	envData := map[string]string{
		"KEY-WITH-HYPHEN": "DASH-VALUE",
		"UNABLE_TO_LOAD":  "VALUE",
	}

	logger := logging.NewMockLogger(logging.DEBUG)

	dir := t.TempDir()

	configFiles := []string{".env", ".local.env"}

	for _, file := range configFiles {
		createEnvFile(t, dir, file, envData)

		env := NewEnvFile(dir, logger)

		assert.Equal(t, "test", env.GetOrDefault("KEY-WITH-HYPHEN", "test"), "TEST Failed.\n godotenv failure with hyphen")
		assert.Empty(t, env.Get("UNABLE_TO_LOAD"), "TEST Failed.\n godotenv failure with hyphen")
	}
}

func createEnvFile(t *testing.T, dir, fileName string, envData map[string]string) {
	t.Helper()

	// Create or open the env file for writing
	envFile, err := os.Create(dir + "/" + fileName)
	if err != nil {
		t.Fatalf("error creating %s file: %v", fileName, err)
	}

	defer envFile.Close()

	// Write data to the env file
	for key, value := range envData {
		_, err := fmt.Fprintf(envFile, "%s=%s\n", key, value)
		if err != nil {
			t.Fatalf("unable to write to file: %v", err)
		}
	}
}
