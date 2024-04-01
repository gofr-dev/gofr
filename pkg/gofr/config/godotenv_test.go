package config

import (
	"fmt"
	"os"
	"testing"

	"gofr.dev/pkg/gofr/testutil"

	"github.com/stretchr/testify/assert"
)

func Test_EnvSuccess(t *testing.T) {
	envData := map[string]string{
		"DATABASE_URL": "localhost:5432",
		"API_KEY":      "your_api_key_here",
		"small_case":   "small_case_value",
	}

	logger := testutil.NewMockLogger(testutil.DEBUGLOG)

	// Call the function to create the .env file
	createEnvFile(t, ".env", envData)

	defer os.RemoveAll("configs")

	env := NewEnvFile("configs", logger)

	assert.Equal(t, "localhost:5432", env.Get("DATABASE_URL"), "TEST Failed.\n godotenv success")
	assert.Equal(t, "your_api_key_here", env.GetOrDefault("API_KEY", "xyz"), "TEST Failed.\n godotenv success")
	assert.Equal(t, "test", env.GetOrDefault("DATABASE", "test"), "TEST Failed.\n godotenv success")
	assert.Equal(t, "small_case_value", env.Get("small_case"), "TEST Failed.\n godotenv success")
}

func Test_EnvSuccess_GofrEnv(t *testing.T) {
	t.Setenv("APP_ENV", "prod")

	envData := map[string]string{
		"DATABASE_URL": "localhost:5432",
		"API_KEY":      "your_api_key_here",
		"small_case":   "small_case_value",
	}

	logger := testutil.NewMockLogger(testutil.DEBUGLOG)

	// Call the function to create the .env file
	createEnvFile(t, ".prod.env", envData)

	defer os.RemoveAll("configs")

	env := NewEnvFile("configs", logger)

	assert.Equal(t, "localhost:5432", env.Get("DATABASE_URL"), "TEST Failed.\n godotenv success")
	assert.Equal(t, "your_api_key_here", env.GetOrDefault("API_KEY", "xyz"), "TEST Failed.\n godotenv success")
	assert.Equal(t, "test", env.GetOrDefault("DATABASE", "test"), "TEST Failed.\n godotenv success")
	assert.Equal(t, "small_case_value", env.Get("small_case"), "TEST Failed.\n godotenv success")
}

func Test_EnvFailureWithHypen(t *testing.T) {
	envData := map[string]string{
		"KEY-WITH-HYPHEN": "DASH-VALUE",
		"UNABLE_TO_LOAD":  "VALUE",
	}

	logger := testutil.NewMockLogger(testutil.DEBUGLOG)

	// Call the function to create the .env file
	createEnvFile(t, ".env", envData)

	defer os.RemoveAll("configs")

	env := NewEnvFile("configs", logger)

	assert.Equal(t, "test", env.GetOrDefault("KEY-WITH-HYPHEN", "test"), "TEST Failed.\n godotenv failure with hyphen")
	assert.Equal(t, "", env.Get("UNABLE_TO_LOAD"), "TEST Failed.\n godotenv failure with hyphen")
}

func createEnvFile(t *testing.T, fileName string, envData map[string]string) {
	err := os.Mkdir("configs", os.ModePerm)
	if err != nil {
		t.Fatalf("unable to create configs directory %v", err)
	}

	// Create or open the .env file for writing
	envFile, err := os.Create("configs/" + fileName)
	if err != nil {
		t.Fatalf("error creating .env file: %v", err)
	}

	defer envFile.Close()

	// Write data to the .env file
	for key, value := range envData {
		_, err := fmt.Fprintf(envFile, "%s=%s\n", key, value)
		if err != nil {
			t.Fatalf("unable to write to file: %v", err)
		}
	}
}
