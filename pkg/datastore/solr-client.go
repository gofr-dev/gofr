package datastore

import (
	"bytes"
	"context"
)

// Client implements the methods to create update search and delete documents
// It also implements methods to retrieve, create, update and delete fields in schema
type Client struct {
	url string
}

// NewSolrClient returns a client to support basic solr functionality
func NewSolrClient(host, port string) Client {
	s := Client{}
	s.url = "http://" + host + ":" + port + "/solr/"

	return s
}

// Document is an interface for managing document-related operations in Solr.
type Document interface {
	Search(ctx context.Context, collection string, params map[string]interface{}) (interface{}, error)
	Create(ctx context.Context, collection string, document *bytes.Buffer, params map[string]interface{}) (interface{}, error)
	Update(ctx context.Context, collection string, document *bytes.Buffer, params map[string]interface{}) (interface{}, error)
	Delete(ctx context.Context, collection string, document *bytes.Buffer, params map[string]interface{}) (interface{}, error)
}

// Schema is an interface for managing schema-related operations in Solr.
type Schema interface {
	Retrieve(ctx context.Context, collection string, params map[string]interface{}) (interface{}, error)
	ListFields(ctx context.Context, collection string, params map[string]interface{}) (interface{}, error)
	AddField(ctx context.Context, collection string, document *bytes.Buffer) (interface{}, error)
	UpdateField(ctx context.Context, collection string, document *bytes.Buffer) (interface{}, error)
	DeleteField(ctx context.Context, collection string, document *bytes.Buffer) (interface{}, error)
}
