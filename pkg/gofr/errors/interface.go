package error

// GoFrErr represents a generic interface for gofr's error.
type GoFrErr interface {
	Error() string
	StatusCode() int
}
