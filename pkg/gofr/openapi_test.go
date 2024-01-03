package gofr

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
)

const openapiJSONString = `{
  "openapi": "3.0.0",
  "info": {
    "version": "1.0.0",
    "title": "Swagger Example Api"
  },
  "paths": {
    "/pets": {
      "get": {
        "summary": "List all pets",
        "operationId": "listPets",
        "tags": [
          "pets"
        ],
        "parameters": [
          {
            "name": "limit",
            "in": "query",
            "description": "How many items to return at one time (max 100)",
            "required": false,
            "schema": {
              "type": "integer",
              "format": "int32"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "A paged array of pets",
            "headers": {
              "x-next": {
                "description": "A link to the next page of responses",
                "schema": {
                  "type": "string"
                }
              }
            },
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Pets"
                }
              }
            }
          },
          "default": {
            "description": "unexpected error",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Error"
                }
              }
            }
          }
        }
      },
      "post": {
        "summary": "Create a pet",
        "operationId": "createPets",
        "tags": [
          "pets"
        ],
        "responses": {
          "201": {
            "description": "Null response"
          },
          "default": {
            "description": "unexpected error",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Error"
                }
              }
            }
          }
        }
      }
    },
    "/pets/{petId}": {
      "get": {
        "summary": "Info for a specific pet",
        "operationId": "showPetById",
        "tags": [
          "pets"
        ],
        "parameters": [
          {
            "name": "petId",
            "in": "path",
            "required": true,
            "description": "The id of the pet to retrieve",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Expected response to a valid request",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Pet"
                }
              }
            }
          },
          "default": {
            "description": "unexpected error",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Error"
                }
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "Pet": {
        "type": "object",
        "required": [
          "id",
          "name"
        ],
        "properties": {
          "id": {
            "type": "integer",
            "format": "int64"
          },
          "name": {
            "type": "string"
          },
          "tag": {
            "type": "string"
          }
        }
      },
      "Pets": {
        "type": "array",
        "items": {
          "$ref": "#/components/schemas/Pet"
        }
      },
      "Error": {
        "type": "object",
        "required": [
          "code",
          "message"
        ],
        "properties": {
          "code": {
            "type": "integer",
            "format": "int32"
          },
          "message": {
            "type": "string"
          }
        }
      }
    }
  }
}`

func createSampleOpenAPI() {
	rootDir, _ := os.Getwd()
	logger := log.NewLogger()
	fileDir := rootDir + "/api"

	_ = os.Mkdir(fileDir, os.ModePerm)

	f, err := os.Create(fileDir + "/openapi.json")
	if err != nil {
		logger.Error(err)
	}

	_, err = f.WriteString(openapiJSONString)

	if err != nil {
		logger.Error(err)
	} else {
		logger.Info("OpenApi created!")
	}

	err = f.Close()

	if err != nil {
		logger.Error(err)
	}
}

func deleteSampleOpenAPI() {
	rootDir, _ := os.Getwd()
	logger := log.NewLogger()

	err := os.RemoveAll(rootDir + "/api")
	if err != nil {
		logger.Error(err)
	}
}

func TestOpenAPIHandler(t *testing.T) {
	g := New()
	// Added contextInjector middleware
	g.Server.Router.Use(g.Server.contextInjector)

	g.GET("/.well-known/openapi.json", OpenAPIHandler)

	createSampleOpenAPI()

	defer deleteSampleOpenAPI()

	url := "http://localhost:3395/.well-known/openapi.json"
	req, _ := http.NewRequest(http.MethodGet, url, http.NoBody)

	w := httptest.NewRecorder()

	g.Server.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("want status code 200 got= %v", w.Code)
	}
}

func TestOpenAPIHandlerError(t *testing.T) {
	g := New()
	// Added contextInjector middleware
	g.Server.Router.Use(g.Server.contextInjector)

	rootDir := t.TempDir()
	path := rootDir + "/" + "api"

	g.GET("/.well-known/openapi.json", OpenAPIHandler)

	url := "http://localhost:3396/.well-known/openapi.json"
	req, _ := http.NewRequest(http.MethodGet, url, http.NoBody)
	w := httptest.NewRecorder()

	g.Server.Router.ServeHTTP(w, req)

	expResp := errors.FileNotFound{
		FileName: "openapi.json",
		Path:     path,
	}

	resp := errors.FileNotFound{}
	data := w.Body.Bytes()
	_ = json.Unmarshal(data, &resp)

	if w.Code != http.StatusNotFound {
		assert.Equal(t, expResp, resp, "TEST Failed.\n")
	}
}

func TestSwaggerUIHandler(t *testing.T) {
	g := New()
	// Added contextInjector middleware
	g.Server.Router.Use(g.Server.contextInjector)

	g.GET("/.well-known/swagger", SwaggerUIHandler)
	g.GET("/.well-known/swagger/{name}", SwaggerUIHandler)

	createSampleOpenAPI()

	defer deleteSampleOpenAPI()

	resourceList := []struct {
		resource      string
		expStatusCode int
	}{
		{"swagger", 200},
		{"swagger/swagger-ui.css", 200},
		{"swagger/favicon-32x32.png", 200},
		{"swagger/favicon-16x16.png", 200},
		{"swagger/swagger-ui-bundle.js", 200},
		{"swagger/swagger-ui-standalone-preset.js", 200},
		{"swagger/some-random-file.txt", 404},
	}

	for _, resource := range resourceList {
		url := "http://localhost:3395/.well-known/" + resource.resource
		req, _ := http.NewRequest(http.MethodGet, url, http.NoBody)

		w := httptest.NewRecorder()

		g.Server.Router.ServeHTTP(w, req)

		if w.Code != resource.expStatusCode {
			t.Errorf("want status code: %v, got= %v for resource : %v", resource.expStatusCode, w.Code, resource.resource)
		}
	}
}
