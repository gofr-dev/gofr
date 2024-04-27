package config

import (
	"fmt"
	"os"

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
	Warnf(format string, a ...interface{})
	Infof(format string, a ...interface{})
	Debugf(format string, a ...interface{})
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

	err := godotenv.Load(defaultFile)
	if err != nil {
		e.logger.Warnf("Failed to load config from file: %v, Err: %v", defaultFile, err)
	} else {
		e.logger.Infof("Loaded config from file: %v", defaultFile)
	}

	switch env {
	case "":
		// If 'APP_ENV' is not set, then GoFr will read '.env' from configs directory, and then it will be overwritten
		// by configs present in file '.local.env'
		err = godotenv.Overload(overrideFile)
		if err != nil {
			e.logger.Debugf("Failed to load config from file: %v, Err: %v", overrideFile, err)
		} else {
			e.logger.Infof("Loaded config from file: %v", overrideFile)
		}

	default:
		// If 'APP_ENV' is set to x, then GoFr will read '.env' from configs directory, and then it will be overwritten
		// by configs present in file '.x.env'
		overrideFile = fmt.Sprintf("%s/.%s.env", folder, env)

		err = godotenv.Overload(overrideFile)
		if err != nil {
			e.logger.Warnf("Failed to load config from file: %v, Err: %v", overrideFile, err)
		} else {
			e.logger.Infof("Loaded config from file: %v", overrideFile)
		}
	}
}

func (e *EnvLoader) Get(key string) string {
	return os.Getenv(key)
}

func (e *EnvLoader) GetOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}

	return defaultValue
}
