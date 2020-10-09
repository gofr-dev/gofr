package config

import (
	"os"

	"github.com/joho/godotenv"
)

type EnvFile struct{}

func NewEnvFile(configFolder string) *EnvFile {
	conf := &EnvFile{}
	conf.read(configFolder)

	return conf
}

func (e *EnvFile) read(folder string) {
	defaultFile := folder + "/.env"

	_ = godotenv.Load(defaultFile)
}
func (e *EnvFile) Get(key string) string {
	return os.Getenv(key)
}

func (e *EnvFile) GetOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}

	return defaultValue
}
