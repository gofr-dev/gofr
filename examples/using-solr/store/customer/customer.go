package customer

import (
	"bytes"
	"encoding/json"
	"strconv"

	"gofr.dev/examples/using-solr/store"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/gofr"
)

type customer struct {
	solr datastore.Document
}

// New is factory function for Store layer
//
//nolint:revive // store should not be used without proper initilization with required dependency
func New(client datastore.Document) customer {
	return customer{solr: client}
}

// List searches customers based on the filters passed by querying to SOLR
func (c customer) List(ctx *gofr.Context, collection string, filter store.Filter) ([]store.Model, error) {
	resp, err := c.solr.Search(ctx, collection, map[string]interface{}{"q": filter.GenSolrQuery(), "wt": "json"})
	if err != nil {
		return nil, err
	}

	res := resp.(datastore.Response)
	solrRes := struct {
		Response struct {
			Docs []struct {
				ID   string `json:"id"`
				Name string `json:"Name"`
				DOB  string `json:"dateOfBirth"`
			} `json:"Docs"`
		}
	}{}
	b, _ := json.Marshal(res.Data)

	err = json.Unmarshal(b, &solrRes)
	if err != nil {
		return nil, err
	}

	customers := make([]store.Model, 0)

	for _, doc := range solrRes.Response.Docs {
		id, _ := strconv.Atoi(doc.ID)

		customers = append(customers, store.Model{ID: id, Name: doc.Name, DateOfBirth: doc.DOB})
	}

	return customers, nil
}

// Create creates a document in the specified collection
func (c customer) Create(ctx *gofr.Context, collection string, customer store.Model) error {
	b, _ := json.Marshal([]store.Model{customer})
	_, err := c.solr.Create(ctx, collection, bytes.NewBuffer(b), map[string]interface{}{"commit": true})

	return err
}

// Update updates a document with id = customer.ID in the specified collection
func (c customer) Update(ctx *gofr.Context, collection string, customer store.Model) error {
	b, _ := json.Marshal([]store.Model{customer})
	_, err := c.solr.Update(ctx, collection, bytes.NewBuffer(b), map[string]interface{}{"commit": true})

	return err
}

// Delete deletes a document whose id = customer.ID in the specified collection
func (c customer) Delete(ctx *gofr.Context, collection string, customer store.Model) error {
	b := struct {
		Delete []string `json:"delete"`
	}{[]string{strconv.Itoa(customer.ID)}}
	body, _ := json.Marshal(b)
	_, err := c.solr.Delete(ctx, collection, bytes.NewBuffer(body), map[string]interface{}{"commit": true})

	return err
}
