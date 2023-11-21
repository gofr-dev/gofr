# Remote Files

GoFr allows us to do file operation.
It supports the following file stores LOCAL filesystem, AWS, AZURE, GCP, FTP and SFTP.

# Usage

To enable usage of filestore add the following configs in .env file.

Lets configure to move files in local filesystem, add the following in `.env` file

```bash
# It can have values LOCAL, AZURE, AWS, GCP, SFTP or FTP
FILE_STORE=LOCAL
```

To move files for a different filestore, change the FILE_STORE variable and add respective
configs by referring [here](/docs/v1/references/configs/page.md).

Implementation will remain same as below, as GoFr abstracts the logic and initialises the respective filestore based on the configs.

```go
package main

import (
	"fmt"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/file"
	"gofr.dev/pkg/gofr"
)

const size = 20

func main() {
	app := gofr.NewCMD()

	fileAbstracter, err := file.NewWithConfig(app.Config, "test.txt", "rw")
	if err != nil {
		app.Logger.Error("Unable to initialize", err)
		return
	}

	app.GET("read", func(c *gofr.Context) (interface{}, error) {
		err := fileAbstracter.Open()
		if err != nil {
			return nil, err
		}

		defer fileAbstracter.Close()

		b := make([]byte, size)

		_, err = fileAbstracter.Read(b)
		if err != nil {
			return nil, err
		}

		return string(b), nil
	})

	app.GET("write", func(c *gofr.Context) (interface{}, error) {
		err := fileAbstracter.Open()
		if err != nil {
			return nil, err
		}

		b := []byte("Welcome to GoFr!")

		_, err = fileAbstracter.Write(b)
		if err != nil {
			return nil, err
		}

		err = fileAbstracter.Close()
		if err != nil {
			return nil, err
		}

		return "File written successfully!", err
	})

	app.GET("list", func(c *gofr.Context) (interface{}, error) {
		k, err := fileAbstracter.List(".")

		return k, err
	})

	app.GET("move", func(ctx *gofr.Context) (interface{}, error) {
		src := ctx.Param("src")
		if src == "" {
			return nil, errors.MissingParam{Param: []string{"src"}}
		}

		dest := ctx.Param("dest")
		if dest == "" {
			return nil, errors.MissingParam{Param: []string{"dest"}}
		}

		err := fileAbstracter.Move(src, dest)
		if err != nil {
			return nil, err
		}

		defer fileAbstracter.Close()

		return fmt.Sprintf("File moved successfully from source:%s to destination:%s", src, dest), nil
	})

	app.Start()
}

```
