// Package config provides the functionality to read environment variables
// it has the power to read from a static config file or from a remote config server
package config

import (
	"os"

	"github.com/joho/godotenv"
)

type GoDotEnvProvider struct {
	// contains unexported fields
	configFolder string
	logger       logger
}

type logger interface {
	Log(args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, a ...interface{})
	Error(args ...interface{})
	Errorf(format string, a ...interface{})
	Info(args ...interface{})
	Infof(format string, a ...interface{})
}

// NewGoDotEnvProvider creates a new instance of GoDotEnvProvider.
func NewGoDotEnvProvider(l logger, configFolder string) *GoDotEnvProvider {
	provider := &GoDotEnvProvider{
		configFolder: configFolder,
		logger:       l,
	}

	provider.readConfig(configFolder)

	return provider
}

// readConfig(logger Logger) loads the environment variables from .env file
// Priority Order is Environment Variable > .X.env file > .env file
// if there is a need to overwrite any of the environment variable present in the ./env
// then it can be done by creating .env.local file
// or by specifying the file prefix in environment variable GOFR_ENV.
func (g *GoDotEnvProvider) readConfig(confLocation string) {
	const env = ".env"

	var (
		defaultFile  = confLocation + "/" + env
		overrideFile = confLocation + "/.local" + env
	)

	gofrEnv := g.Get("GOFR_ENV")
	if gofrEnv != "" {
		overrideFile = confLocation + "/." + gofrEnv + env
	}

	if err := godotenv.Load(overrideFile); err == nil {
		g.logger.Log("Loaded config from file: ", overrideFile)
	} else if gofrEnv != "" { // log an error if gofr env is set and the file could not be loaded
		g.logger.Warnf("Failed to load config from file: %v, Err: %v", overrideFile, err)
	}

	if err := godotenv.Load(defaultFile); err != nil {
		g.logger.Warnf("Failed to load config from file: %v, Err: %v", defaultFile, err)
	} else {
		g.logger.Log("Loaded config from file: ", defaultFile)
	}
}

// Get retrieves the value of an environment variable by its key.
func (g *GoDotEnvProvider) Get(key string) string {
	return os.Getenv(key)
}

// GetOrDefault retrieves the value of an environment variable by its key, or returns a default value
// if the variable is not set.
func (g *GoDotEnvProvider) GetOrDefault(key, defaultValue string) string {
	val := os.Getenv(key)
	if val != "" {
		return val
	}

	return defaultValue
}
