package config

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/logging"
)

func Test_EnvSuccess(t *testing.T) {
	envData := map[string]string{
		"DATABASE_URL": "localhost:5432",
		"API_KEY":      "your_api_key_here",
		"small_case":   "small_case_value",
	}

	logger := logging.NewMockLogger(logging.DEBUG)

	err := createConfigsDirectory()
	if err != nil {
		t.Error(err)
	}

	// Call the function to create the .env file
	createEnvFile(t, ".env", envData)

	defer os.RemoveAll("configs")

	env := NewEnvFile("configs", logger)

	assert.Equal(t, "localhost:5432", env.Get("DATABASE_URL"), "TEST Failed.\n godotenv success")
	assert.Equal(t, "your_api_key_here", env.GetOrDefault("API_KEY", "xyz"), "TEST Failed.\n godotenv success")
	assert.Equal(t, "test", env.GetOrDefault("DATABASE", "test"), "TEST Failed.\n godotenv success")
	assert.Equal(t, "small_case_value", env.Get("small_case"), "TEST Failed.\n godotenv success")
}

func Test_EnvSuccess_AppEnv_Override(t *testing.T) {
	t.Setenv("APP_ENV", "prod")

	envData := map[string]string{
		"DATABASE_URL": "localhost:5432",
	}

	err := createConfigsDirectory()
	if err != nil {
		t.Error(err)
	}

	// Call the function to create the .env file
	createEnvFile(t, ".env", envData)

	// override database url in '.prod.env' file to test if value if being overridden
	createEnvFile(t, ".prod.env", map[string]string{"DATABASE_URL": "localhost:2001"})

	logger := logging.NewMockLogger(logging.DEBUG)

	env := NewEnvFile("configs", logger)

	defer os.RemoveAll("configs")

	assert.Equal(t, "localhost:2001", env.Get("DATABASE_URL"), "TEST Failed.\n godotenv success")
}

func Test_EnvSuccess_Local_Override(t *testing.T) {
	t.Setenv("APP_ENV", "")

	envData := map[string]string{
		"API_KEY": "your_api_key_here",
	}

	err := createConfigsDirectory()
	if err != nil {
		t.Error(err)
	}

	// Call the function to create the .env file
	createEnvFile(t, ".env", envData)

	// override database url in '.prod.env' file to test if value if being overridden
	createEnvFile(t, ".local.env", map[string]string{"API_KEY": "overloaded_api_key"})

	logger := logging.NewMockLogger(logging.DEBUG)

	env := NewEnvFile("configs", logger)

	defer os.RemoveAll("configs")

	assert.Equal(t, "overloaded_api_key", env.Get("API_KEY"), "TEST Failed.\n godotenv success")
}

func Test_EnvFailureWithHyphen(t *testing.T) {
	envData := map[string]string{
		"KEY-WITH-HYPHEN": "DASH-VALUE",
		"UNABLE_TO_LOAD":  "VALUE",
	}

	logger := logging.NewMockLogger(logging.DEBUG)

	err := createConfigsDirectory()
	if err != nil {
		t.Error(err)
	}

	defer os.RemoveAll("configs")

	configFiles := []string{".env", ".local.env"}

	for _, file := range configFiles {
		createEnvFile(t, file, envData)

		env := NewEnvFile("configs", logger)

		assert.Equal(t, "test", env.GetOrDefault("KEY-WITH-HYPHEN", "test"), "TEST Failed.\n godotenv failure with hyphen")
		assert.Equal(t, "", env.Get("UNABLE_TO_LOAD"), "TEST Failed.\n godotenv failure with hyphen")
	}
}

func createEnvFile(t *testing.T, fileName string, envData map[string]string) {
	t.Helper()

	// Create or open the env file for writing
	envFile, err := os.Create("configs/" + fileName)
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

func createConfigsDirectory() error {
	err := os.Mkdir("configs", os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}
