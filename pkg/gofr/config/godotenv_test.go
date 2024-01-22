package config

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func Test_EnvSuccess(t *testing.T) {
	envData := map[string]string{
		"DATABASE_URL": "localhost:5432",
		"API_KEY":      "your_api_key_here",
		"small_case":   "small_case_value",
	}

	// Call the function to create the .env file
	err := createEnvFile(t, envData)
	if err != nil {
		t.Fatalf("Error creating env file : %v", err)
	}

	defer os.RemoveAll("configs")

	env := NewEnvFile("configs")

	assert.Equal(t, "localhost:5432", env.Get("DATABASE_URL"))
	assert.Equal(t, "your_api_key_here", env.GetOrDefault("API_KEY", "xyz"))
	assert.Equal(t, "test", env.GetOrDefault("DATABASE", "test"))
	assert.Equal(t, "small_case_value", env.Get("small_case"))
}

func Test_EnvFailureWithHypen(t *testing.T) {
	envData := map[string]string{
		"KEY-WITH-HYPHEN": "DASH-VALUE",
		"UNABLE_TO_LOAD":  "VALUE",
	}

	// Call the function to create the .env file
	err := createEnvFile(t, envData)
	if err != nil {
		t.Fatalf("Error creating env file : %v", err)
	}

	defer os.RemoveAll("configs")

	env := NewEnvFile("configs")

	assert.Equal(t, "test", env.GetOrDefault("KEY-WITH-HYPHEN", "test"))
	assert.Equal(t, "", env.Get("UNABLE_TO_LOAD"))
}

func createEnvFile(t *testing.T, envData map[string]string) error {
	err := os.Mkdir("configs", os.ModePerm)
	if err != nil {
		t.Fatalf("unable to create configs directory %v", err)
	}

	// Create or open the .env file for writing
	envFile, err := os.Create("configs/.env")
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

	return nil
}
