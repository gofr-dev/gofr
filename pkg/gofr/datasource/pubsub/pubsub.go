package pubsub

import (
	"gofr.dev/pkg/gofr/config"
)

func New(conf config.Config, logger Logger) Client {
	switch conf.Get("PUBSUB_BACKEND") {

	}

	return nil
}
