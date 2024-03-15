package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/file"
)

func main() {
	app := gofr.New()

	app.POST("/upload", func(c *gofr.Context) (interface{}, error) {
		var data = struct {
			File *file.Zip `file:"file"`
		}{}

		err := c.Bind(&data)
		if err != nil {
			return nil, err
		}

		if data.File == nil {
			return "no file was uploaded", nil
		}

		err = data.File.CreateLocalCopies("tmp")
		if err != nil {
			return nil, err
		}

		return "Created", nil
	})

	app.Run()
}
