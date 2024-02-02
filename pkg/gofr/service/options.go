package service

type Options interface {
	addOption(h HTTP) HTTP
}
