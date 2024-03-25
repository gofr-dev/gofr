package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type EnvFileProvider struct {
	logger logger
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

func NewEnvFile(configFolder string, logger logger) *EnvFileProvider {
	conf := &EnvFileProvider{logger: logger}
	conf.read(configFolder)

	return conf
}

func (e *EnvFileProvider) read(folder string) {
	var defaultFile = folder + "/.env"

	overrideFile := func() string {
		gofrEnv := e.Get("GOFR_ENV")
		if gofrEnv != "" {
			return fmt.Sprintf("%s/.%s.env", folder, gofrEnv)
		}

		return fmt.Sprintf("%s/.local.env", folder)
	}()

	// If 'GOFR_ENV' is set to x, then GoFr will try to read '.x.env' file from configs directory, if failed it will read
	// the default file '.env'.
	// If 'GOFR_ENV' is not set , GoFr will first try to read '.local.env', if failed then it
	// will read default env file.
	if err := godotenv.Load(overrideFile); err != nil {
		e.logger.Warnf("Failed to load config from file: %v, Err: %v", overrideFile, err)
	} else {
		e.logger.Log("Loaded config from file: ", overrideFile)

		return
	}

	if err := godotenv.Load(defaultFile); err != nil {
		e.logger.Warnf("Failed to load config from file: %v, Err: %v", defaultFile, err)
	} else {
		e.logger.Log("Loaded config from file: ", defaultFile)
	}
}

func (e *EnvFileProvider) Get(key string) string {
	return os.Getenv(key)
}

func (e *EnvFileProvider) GetOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}

	return defaultValue
}
