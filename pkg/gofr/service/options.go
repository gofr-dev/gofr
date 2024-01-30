package service

type Options interface {
	apply(h HTTP) HTTP
}
