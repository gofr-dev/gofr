package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	app.POST("/dir", CreateDir)
	app.PUT("/dir", Put)

	app.POST("/file", Create)
	app.PUT("/file", Update)
	app.GET("/file", Get)

	app.DELETE("/file", Delete)

	app.Run()
}

func CreateDir(c *gofr.Context) (interface{}, error) {
	err := c.File.CreateDir("temp")
	if err != nil {
		return nil, err
	}

	return "Directory Created Successfully", nil
}

func Create(c *gofr.Context) (interface{}, error) {
	err := c.File.Create("temp.txt", []byte("This is test content"))
	if err != nil {
		return nil, err
	}

	return "File has been created", nil
}

func Update(c *gofr.Context) (interface{}, error) {
	err := c.File.Update("temp.txt", []byte("I have been updated"))
	if err != nil {
		return nil, err
	}

	return "File content has been updated", nil
}

func Get(c *gofr.Context) (interface{}, error) {
	content, err := c.File.Read("temp.txt")
	if err != nil {
		return nil, err
	}

	filInfo, err := c.File.Stat("temp.txt")
	if err != nil {
		return nil, err
	}

	return struct {
		Name    string
		Size    int64
		Content string
	}{
		Name:    filInfo.Name(),
		Size:    filInfo.Size(),
		Content: string(content),
	}, nil
}

func Put(c *gofr.Context) (interface{}, error) {
	err := c.File.Move("temp.txt", "temp/temp.txt")
	if err != nil {
		return nil, err
	}

	return "File moved inside directory", nil
}

func Delete(c *gofr.Context) (interface{}, error) {
	err := c.File.Delete("temp")
	if err != nil {
		return nil, err
	}

	return "File has been deleted", nil
}
