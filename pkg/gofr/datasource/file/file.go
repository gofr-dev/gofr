package file

import (
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource/file/ftp"
	"gofr.dev/pkg/gofr/datasource/file/local"
	"strings"
)

func New(config config.Config) File {
	switch strings.ToLower(config.Get("FILE_SYSTEM")) {

	case "local":
		return local.New()
	case "ftp":
		return ftp.New(config)
	}

	return nil
}
