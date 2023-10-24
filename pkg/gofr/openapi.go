package gofr

import (
	"os"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/template"
	"gofr.dev/web"
)

// OpenAPIHandler serves the openapi.json file present either in the root directory or in root/api directory
func OpenAPIHandler(*Context) (interface{}, error) {
	rootDir, _ := os.Getwd()
	fileDir := rootDir + "/" + "api"

	return template.Template{Directory: fileDir, File: "openapi.json", Data: nil, Type: template.FILE}, nil
}

// SwaggerUIHandler handles requests for Swagger UI files
func SwaggerUIHandler(c *Context) (interface{}, error) {
	fileName := c.PathParam("name")

	data, contentType, err := web.GetSwaggerFile(fileName)
	// error returned will only be of type &&fs.PathError
	if err != nil {
		return nil, errors.FileNotFound{
			FileName: fileName,
		}
	}

	return template.File{Content: data, ContentType: contentType}, nil
}
