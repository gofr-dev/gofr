package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/file"
)

func main() {
	app := gofr.New()

	app.POST("/upload", UploadHandler)

	app.Run()
}

// Data is the struct that we are trying to bind files to
type Data struct {
	// The Compressed field is of type zip,
	// the tag `upload` signifies the key for the form where the file is uploaded
	// if the tag is not present, the field name would be taken as a key.
	Compressed file.Zip `file:"upload"`
}

func UploadHandler(c *gofr.Context) (interface{}, error) {
	var d Data

	// bind the multipart data into the variable d
	err := c.Bind(&d)
	if err != nil {
		return nil, err
	}

	// create local copies of the zipped files in tmp folder
	err = d.Compressed.CreateLocalCopies("tmp")
	if err != nil {
		return nil, err
	}

	// return the number of compressed files recieved
	return len(d.Compressed.Files), nil
}
