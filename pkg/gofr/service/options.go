package service

type Options interface {
	addOption(h HTTPService) HTTPService
}
