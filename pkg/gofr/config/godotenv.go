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
	// Capture initial system environment keys
	initialEnv := e.captureInitialEnv()

	// Capture APP_ENV before loading files to ensure proper environment-specific file loading
	appEnv := os.Getenv("APP_ENV")

	// Load environment files with proper precedence
	envMap := e.loadEnvironmentFiles(folder, appEnv)

	// Apply to environment variables, respecting system env precedence
	e.applyEnvironmentVariables(envMap, initialEnv)
}

// captureInitialEnv captures the initial system environment keys.
func (*EnvLoader) captureInitialEnv() map[string]bool {
	initialEnv := make(map[string]bool)

	for _, envVar := range os.Environ() {
		key, _, _ := strings.Cut(envVar, "=")
		initialEnv[key] = true
	}

	return initialEnv
}

// loadEnvironmentFiles loads all environment files with proper precedence.
func (e *EnvLoader) loadEnvironmentFiles(folder, appEnv string) map[string]string {
	envMap := make(map[string]string)

	// Load base .env file (lowest precedence)
	e.loadBaseEnvFile(folder, envMap)

	// Load default override (.local.env) if exists (medium precedence)
	e.loadLocalOverrideFile(folder, envMap)

	// Load app-env specific override (highest file precedence)
	e.loadEnvSpecificFile(folder, envMap, appEnv)

	return envMap
}

// loadBaseEnvFile loads the base .env file.
func (e *EnvLoader) loadBaseEnvFile(folder string, envMap map[string]string) {
	defaultFile := folder + defaultFileName

	if content, err := godotenv.Read(defaultFile); err == nil {
		for k, v := range content {
			envMap[k] = v
		}

		e.logger.Infof("Loaded config from file: %v", defaultFile)
	} else if !errors.Is(err, fs.ErrNotExist) {
		// CRITICAL: Fatal error for non-file-not-found errors (permissions, corruption, etc.)
		e.logger.Fatalf("Failed to load config from file: %v, Err: %v", defaultFile, err)
	}
}

// loadLocalOverrideFile loads the .local.env override file.
func (e *EnvLoader) loadLocalOverrideFile(folder string, envMap map[string]string) {
	localOverridePath := folder + defaultOverrideFileName

	if content, err := godotenv.Read(localOverridePath); err == nil {
		for k, v := range content {
			envMap[k] = v
		}

		e.logger.Debugf("Applied override config: %v", localOverridePath)
	}
}

// loadEnvSpecificFile loads the app-env specific override file.
func (e *EnvLoader) loadEnvSpecificFile(folder string, envMap map[string]string, appEnv string) {
	if appEnv == "" {
		return
	}

	envSpecificFile := fmt.Sprintf("%s/.%s.env", folder, appEnv)

	if content, err := godotenv.Read(envSpecificFile); err == nil {
		for k, v := range content {
			envMap[k] = v
		}

		e.logger.Debugf("Applied app-env override config: %v", envSpecificFile)
	} else if !errors.Is(err, fs.ErrNotExist) {
		// CRITICAL: Fatal error for non-file-not-found errors (permissions, corruption, etc.)
		e.logger.Fatalf("Failed to load config from file: %v, Err: %v", envSpecificFile, err)
	}
}

// applyEnvironmentVariables applies environment variables respecting system precedence.
func (*EnvLoader) applyEnvironmentVariables(envMap map[string]string, initialEnv map[string]bool) {
	for key, value := range envMap {
		// Only set if not in initial system environment
		if !initialEnv[key] {
			os.Setenv(key, value)
		}
	}
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
