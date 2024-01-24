package service

type Options interface {
	apply(h *httpService, logger Logger)
}
