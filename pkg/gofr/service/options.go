package service

type Options interface {
	AddOption(h HTTP) HTTP
}
