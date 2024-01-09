package config

type Config interface {
	Get(string) string
	GetOrDefault(string, string) string
}
