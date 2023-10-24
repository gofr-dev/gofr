package config

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/log"
)

// This Test is to check if environment variables are loaded from configs/.env on
// initialization of new gofr object
func TestGet_Read_Values_From_Config_Files(t *testing.T) {
	t.Setenv("GOFR_ENV", "test")

	testCases := []struct {
		desc     string
		envKey   string
		envValue string
	}{
		{"Success Case: Load from original .env file", "TEST", "test"},
		{"Success Case: Load from .test.env file", "EXAMPLE", "example"},
		{"Success Case: The key is present in both but value should be from .test.env", "OVERWRITE", "success"},
	}

	conf := NewGoDotEnvProvider(log.NewMockLogger(io.Discard), "../../../configs")

	for _, tc := range testCases {
		val := conf.Get(tc.envKey)
		if val != tc.envValue {
			t.Errorf("Test Failed.\t Expected env value: %v\tGot env value: %v", tc.envValue, val)
		}
	}
}

func TestNewGoDotEnvProvider(t *testing.T) {
	logger := log.NewMockLogger(new(bytes.Buffer))

	os.Unsetenv("APP_NAME")
	os.Unsetenv("GOFR_ENV")

	// testing case where folder doesn't exist
	c := NewGoDotEnvProvider(logger, "./configs")

	if app := c.Get("APP_NAME"); app != "" {
		t.Errorf("FAILED, Expected: %s, Got: %s", "", app)
	}

	// testing case where folder does exist
	NewGoDotEnvProvider(logger, "../../../configs")

	expected := "gofr"

	if got := os.Getenv("APP_NAME"); got != expected {
		t.Errorf("FAILED, Expected: %s, Got: %s", expected, got)
	}
}

// Test_readConfig to test readConfig
func Test_readConfig(t *testing.T) {
	const envTest = "test"

	configPath := "../../../configs"

	testCases := []struct {
		desc         string
		configFolder string
		env          string
		logMessage   string
	}{
		{"Success : .env file exists", configPath, envTest, "Loaded config from file:  ../../../configs/.env"},
		{"Failure: .test.env file exists", configPath, envTest, "Loaded config from file:  ../../../configs/.test.env"},
		{"Error : Invalid Path", "./configs", envTest, "Failed to load config from file: ./configs/.env"},
		{"Failure : .testing.env file doesn't exist", configPath, "testing", "Failed to load config from file: ../../../configs/.testing.env"},
		{"Success : .local.env is present", configPath, "", "Loaded config from file:  ../../../configs/.local.env"},
	}

	for i, tc := range testCases {
		t.Setenv("GOFR_ENV", tc.env)

		b := new(bytes.Buffer)
		logger := log.NewMockLogger(b)

		conf := NewGoDotEnvProvider(logger, tc.configFolder)

		conf.readConfig(tc.configFolder)

		assert.Contains(t, b.String(), tc.logMessage, "Test[%d] Failed: %v", i+1, tc.desc)
	}
}

func TestGetOrDefault(t *testing.T) {
	var (
		key   = "random123"
		value = "value123"
		g     = new(GoDotEnvProvider)
	)

	t.Setenv(key, value)

	if got := g.GetOrDefault(key, "default"); got != value {
		t.Errorf("FAILED, Expected: %v, Got: %v", value, got)
	}

	got := g.GetOrDefault("someKeyThatDoesntExist", "default")
	if got != "default" {
		t.Errorf("FAILED, Expected: default, Got: %v", got)
	}
}

// Test_readConfig_local_env_missing to test when .local.env file is not present
func Test_readConfig_local_env_missing(t *testing.T) {
	t.Setenv("GOFR_ENV", "")

	var (
		configFolder = t.TempDir()
		logMessage   = "Failed to load config from file: " + configFolder + "/.local.env"
	)

	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	conf := NewGoDotEnvProvider(logger, configFolder)

	conf.readConfig(configFolder)
	assert.NotContains(t, b.String(), logMessage, "Test Failed")
}

func TestGet(t *testing.T) {
	testcase := []struct {
		desc     string
		envs     map[string]string
		key      string
		expected string
	}{
		{"GOFR_ENV is not set and config key is present in .local.env", nil, "TEST_ENV_VAL", "test_env_value"},
		{"GOFR_ENV is not set and config key is present in .env", nil, "SOLR_HOST", "localhost"},
		{"Config Key is Not Present", nil, "RANDOM_VAL", ""},
		{"Config Key is Present in .env", nil, "LOG_LEVEL", "INFO"},
		{"Config Key is Present in .test.env", map[string]string{"GOFR_ENV": "test"}, "TEST_VAL", "5050"},
		{"Config key is present as system env.", map[string]string{"TEST_ENV": "random_value"}, "TEST_ENV", "random_value"},
		{".env file has the key but no value", nil, "TEST_CONFIG", ""},
	}

	logger := log.NewMockLogger(io.Discard)

	for i, tc := range testcase {
		// set environment variable
		for k, v := range tc.envs {
			t.Setenv(k, v)
		}

		g := NewGoDotEnvProvider(logger, "../../../configs")
		got := g.Get(tc.key)
		assert.Equal(t, tc.expected, got, "TestCase [%d]: %v, Failed. Expected: %v, Got: %v", i, tc.desc, tc.expected, got)
	}
}

func TestGet_incorrect_configFile_format(t *testing.T) {
	dir := t.TempDir()
	file, _ := os.Create(dir + "/.env")

	defer file.Close()

	_, err := file.WriteString("INVALID_LINE:=\nKEY")
	if err != nil {
		t.Fatal(err)
	}

	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	expectedLog := "Failed to load config from file:"

	g := NewGoDotEnvProvider(logger, dir)

	got := g.Get("RANDOM_ENV")
	if got != "" {
		t.Errorf("expected no value\t got %v", got)
	}

	if !strings.Contains(b.String(), expectedLog) {
		t.Errorf("FAILED. expected log %v\n got %v", expectedLog, b.String())
	}
}

func TestLogsGetConfig(t *testing.T) {
	dir := t.TempDir()
	file, _ := os.Create(dir + "/.env")

	defer file.Close()

	_, err := file.WriteString("TEST_CONFIG=VAL")
	if err != nil {
		t.Fatal(err)
	}

	testcase := []struct {
		desc        string
		envs        map[string]string
		expectedLog string
	}{
		{"Load from .env", map[string]string{"GOFR_ENV": ""}, "Loaded config from file:  ../../../configs/.env"},
		{"Load from .local.env", map[string]string{"GOFR_ENV": ""}, "Loaded config from file:  ../../../configs/.local.env"},
		{"GOFR_ENV set to stage but file not present", map[string]string{"GOFR_ENV": "stage"},
			"Failed to load config from file: ../../../configs/.stage.env"},
		{"GOFR_ENV set to test", map[string]string{"GOFR_ENV": "test"}, "Loaded config from file:  ../../../configs/.test.env"},
	}

	for i, tc := range testcase {
		b := new(bytes.Buffer)
		logger := log.NewMockLogger(b)

		// set environment variable
		for k, v := range tc.envs {
			t.Setenv(k, v)
		}

		_ = NewGoDotEnvProvider(logger, "../../../configs")

		if !strings.Contains(b.String(), tc.expectedLog) {
			t.Errorf("[TESTCASE%d]FAILED. expected log %v\n got %v", i+1, tc.expectedLog, b.String())
		}
	}
}
