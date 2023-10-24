package avro

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/service"
)

// SchemaRegistryClientInterface defines the api for all clients interfacing with schema registry
type SchemaRegistryClientInterface interface {
	GetSchemaByVersion(subject, version string) (int, string, error)
	GetSchema(id int) (string, error)
}

// SchemaRegistryClient is a basic http client to interact with schema registry
type SchemaRegistryClient struct {
	SchemaRegistryConnect []string
	retries               int
	httpSvc               []service.HTTP
	data                  map[int]string
}

type schemaResponse struct {
	ID     int    `json:"id"`
	Schema string `json:"schema"`
}

// NewSchemaRegistryClient creates a client to talk with the schema registry at the connect string
// By default it will retry failed requests (5XX responses and http errors) len(connect) number of times
func NewSchemaRegistryClient(connect []string, user, pass string) SchemaRegistryClientInterface {
	svc := make([]service.HTTP, 0, len(connect))

	const clientTimeout = 2

	options := &service.Options{
		Auth: &service.Auth{
			UserName: user,
			Password: pass,
		},
		SurgeProtectorOption: &service.SurgeProtectorOption{Disable: true},
	}

	for _, v := range connect {
		newSvc := service.NewHTTPServiceWithOptions(v, nil, options)
		newSvc.Timeout = clientTimeout * time.Second // setting the http client timeout to 2 seconds
		svc = append(svc, newSvc)
	}

	return &SchemaRegistryClient{connect, len(connect), svc, make(map[int]string)}
}

// GetSchema returns a schema by unique id
func (client *SchemaRegistryClient) GetSchema(id int) (string, error) {
	if client.data[id] != "" {
		return client.data[id], nil
	}

	resp, err := client.httpCall(fmt.Sprintf("schemas/ids/%d", id))
	if err != nil {
		return "", err
	}

	schema, err := parseSchema(resp)
	if err != nil {
		return "", err
	}

	client.data[id] = schema.Schema

	return schema.Schema, nil
}

// GetSchemaByVersion returns a schema by version
func (client *SchemaRegistryClient) GetSchemaByVersion(subject, version string) (id int, schema string, err error) {
	resp, err := client.httpCall(fmt.Sprintf("subjects/%v/versions/%v", subject, version))
	if err != nil {
		return 0, "", err
	}

	parsedSchema, err := parseSchema(resp)
	if err != nil {
		return 0, "", err
	}

	return parsedSchema.ID, parsedSchema.Schema, err
}

func parseSchema(str []byte) (*schemaResponse, error) {
	var schema = new(schemaResponse)
	err := json.Unmarshal(str, &schema)

	return schema, err
}

func (client *SchemaRegistryClient) httpCall(uri string) ([]byte, error) {
	nServers := len(client.SchemaRegistryConnect)
	offset := rand.Intn(nServers) //nolint:gosec //  Use of weak random number generator

	contentType := "application/vnd.schemaregistry.v1+json"

	for i := 0; ; i++ {
		resp, err := client.httpSvc[(i+offset)%nServers].GetWithHeaders(context.TODO(), uri, nil,
			map[string]string{"Content-Type": contentType})

		forbiddenRequestErr := checkForbiddenRequest(resp, uri)
		if forbiddenRequestErr != nil {
			return nil, forbiddenRequestErr
		}

		if i < client.retries && err != nil {
			continue
		}

		if err != nil {
			return nil, service.FailedRequest{URL: uri, Err: err}
		}

		return resp.Body, nil
	}
}

func checkForbiddenRequest(response *service.Response, uri string) error {
	if response != nil && response.StatusCode == http.StatusForbidden {
		return errors.ForbiddenRequest{URL: uri}
	}

	return nil
}
