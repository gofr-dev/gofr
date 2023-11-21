# Static File Server

To serve static files in gofr you need to return `template.Template` type from your handler.

## Usage

Add the file you want to return in static directory of project's root.

Create `main.go` file and add the following code.

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/template"
)

func main() {
	app := gofr.New()

	app.GET("/file", FileHandler)

	app.Start()
}

func FileHandler(ctx *gofr.Context) (interface{}, error) {
	return template.Template{
        // filename, it reads in the default static directory
		File: "gofr.png",
        // return type
		Type: template.FILE,
	}, nil
}
```

### Changing the Default Directory

Add the directory from where you want to read from, it will fetch relatively from the project's root.

```go
template.Template{
	Directory: "my_directory",
	File: "gofr.png",
	Type: template.FILE,
}
```
