package file

import (
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/file/local"
	"strings"
)

func New(config config.Config, logger datasource.Logger) File {
	switch strings.ToLower(config.Get("FILE_SYSTEM")) {

	case "local":
		return local.New(logger)
	case "ftp":
		//return ftp.New(config)
	}

	return nil
}
