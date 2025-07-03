package cache

type Cache interface {
	Get(key string) (interface{}, bool)

	Set(key string, value interface{})

	Delete(key string)

	Exists(key string) bool

	Clear()

	Close()
}
