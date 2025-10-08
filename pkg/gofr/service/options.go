package service

type Options interface {
	AddOption(h HTTP) HTTP
}

// AddOptionWithValidation extends Options with error reporting capability
type AddOptionWithValidation interface {
	AddOptionWithValidation(h HTTP) (HTTP, error)
}
