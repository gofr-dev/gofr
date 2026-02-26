package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

const (
	defaultFileName         = "/.env"
	defaultOverrideFileName = "/.local.env"
)

type EnvLoader struct {
	logger logger
}

type logger interface {
	Warnf(format string, a ...any)
	Infof(format string, a ...any)
	Debugf(format string, a ...any)
	Fatalf(format string, a ...any)
}

func NewEnvFile(configFolder string, logger logger) Config {
	conf := &EnvLoader{logger: logger}
	conf.read(configFolder)

	return conf
}

func (e *EnvLoader) read(folder string) {
	var (
		defaultFile  = folder + defaultFileName
		overrideFile = folder + defaultOverrideFileName
		env          = e.Get("APP_ENV")
	)

	// Only Capture initial system environment before any file loading
	initialEnv := e.captureInitialEnv()

	err := godotenv.Load(defaultFile)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			e.logger.Fatalf("Failed to load config from file: %v, Err: %v", defaultFile, err)
		}

		e.logger.Warnf("Failed to load config from file: %v, Err: %v", defaultFile, err)
	} else {
		e.logger.Infof("Loaded config from file: %v", defaultFile)
	}

	if env != "" {
		// If 'APP_ENV' is set to x, then GoFr will read '.env' from configs directory, and then it will be overwritten
		// by configs present in file '.x.env'
		overrideFile = fmt.Sprintf("%s/.%s.env", folder, env)
	}

	err = godotenv.Overload(overrideFile)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			e.logger.Fatalf("Failed to load config from file: %v, Err: %v", overrideFile, err)
		}
	} else {
		e.logger.Infof("Loaded config from file: %v", overrideFile)
	}

	// Reload system environment variables to ensure they take precedence over values loaded from files.
	// This is required because Overload replaces the original system variables, which we need to restore.
	for key, envVar := range initialEnv {
		os.Setenv(key, envVar)
	}
}

// captureInitialEnv captures the initial system environment keys.
func (*EnvLoader) captureInitialEnv() map[string]string {
	initialEnv := make(map[string]string)

	for _, envVar := range os.Environ() {
		key, value, found := strings.Cut(envVar, "=")
		if found {
			initialEnv[key] = value
		}
	}

	return initialEnv
}

func (*EnvLoader) Get(key string) string {
	return os.Getenv(key)
}

func (*EnvLoader) GetOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}

	return defaultValue
}
