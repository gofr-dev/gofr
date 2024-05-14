package mongo

type Config interface {
	Get(key string) string
	GetOrDefault(key, def string) string
}
