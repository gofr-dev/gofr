package main

import (
	"io"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/file/s3"
)

func main() {
	app := gofr.New()

	// Register an S3-compatible object store as the file store. The MinIO
	// flavor selects path-style addressing and the checksum handling that
	// self-hosted S3 servers expect.
	app.AddFileStore(s3.New(&s3.Config{
		EndPoint:        app.Config.Get("S3_ENDPOINT"),
		BucketName:      app.Config.Get("S3_BUCKET_NAME"),
		Region:          app.Config.Get("S3_REGION"),
		AccessKeyID:     app.Config.Get("S3_ACCESS_KEY_ID"),
		SecretAccessKey: app.Config.Get("S3_SECRET_ACCESS_KEY"),
		Flavor:          s3.FlavorMinIO,
	}))

	app.POST("/files", uploadHandler)
	app.GET("/files", downloadHandler)

	app.Run()
}

// fileContent is the JSON envelope used to move an object's bytes over HTTP.
type fileContent struct {
	Content string `json:"content"`
}

// uploadHandler writes the posted content to the object named by the "name"
// query parameter. The key may contain "/" separators; S3 needs no parent
// "directory" to exist first.
func uploadHandler(c *gofr.Context) (any, error) {
	name := c.Param("name")

	var in fileContent
	if err := c.Request.Bind(&in); err != nil {
		return nil, err
	}

	f, err := c.File.Create(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err = f.Write([]byte(in.Content)); err != nil {
		return nil, err
	}

	return "uploaded " + name, nil
}

// downloadHandler reads the whole object named by the "name" query parameter
// back to the caller. io.ReadAll drives the file's Read method to completion,
// so the full object must round-trip regardless of its size.
func downloadHandler(c *gofr.Context) (any, error) {
	name := c.Param("name")

	f, err := c.File.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return fileContent{Content: string(data)}, nil
}
