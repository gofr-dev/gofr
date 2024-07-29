package queryparam

// QueryParams interface for handling query parameters.
type QueryParams interface {
	Get(string) string      // Get returns a single value corresponding to provided key.
	GetAll(string) []string // GetAll returns multiple values corresponding to provided key.
}
