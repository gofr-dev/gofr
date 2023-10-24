package customer

import (
	"bytes"
	"net/http"
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/examples/using-elasticsearch/model"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
)

const index = "customers"

func TestMain(m *testing.M) {
	app := gofr.New()

	const mapping = `{"settings": {"number_of_shards": 1},"mappings": {"_doc": {"properties": {
				"id": {"type": "text"},"name": {"type": "text"},"city": {"type": "text"}}}}}`

	es := app.Elasticsearch
	_, err := es.Indices.Create(index,
		es.Indices.Create.WithBody(bytes.NewReader([]byte(mapping))),
		es.Indices.Create.WithPretty(),
	)

	if err != nil {
		app.Logger.Errorf("error creating index: %s", err.Error())
	}

	seeder := datastore.NewSeeder(&app.DataStore, "../../db")
	seeder.RefreshElasticSearch(app.Logger, index)

	os.Exit(m.Run())
}

func initializeElasticsearchClient() (*store, *gofr.Context) {
	store := New(index)

	app := gofr.New()
	req, _ := http.NewRequest(http.MethodGet, "/customers/_search", http.NoBody)
	r := request.NewHTTPRequest(req)
	ctx := gofr.NewContext(nil, r, app)
	ctx.Context = req.Context()

	return &store, ctx
}

// creating index 'customers' and populating data from .csv file to use it in tests
func initializeTests(t *testing.T) *gofr.Context {
	app := gofr.New()

	seeder := datastore.NewSeeder(&app.DataStore, "../../db")
	seeder.RefreshElasticSearch(t, index)

	req, _ := http.NewRequest(http.MethodGet, "/customers/_search", http.NoBody)
	r := request.NewHTTPRequest(req)
	ctx := gofr.NewContext(nil, r, app)
	ctx.Context = req.Context()

	return ctx
}

func TestCustomer_Get(t *testing.T) {
	tests := []struct {
		desc string
		name string
		resp []model.Customer
	}{
		{"get invalid name", "2111", nil},
		{"get success", "Henry", []model.Customer{{ID: "1", Name: "Henry", City: "Bangalore"}}},
		{"get non existent entity", "Random", nil},
		{"get multiple entities", "", []model.Customer{
			{ID: "1", Name: "Henry", City: "Bangalore"},
			{ID: "2", Name: "Bitsy", City: "Mysore"},
			{ID: "3", Name: "Magic", City: "Bangalore"},
		}},
	}

	store := New(index)
	ctx := initializeTests(t)

	for i, tc := range tests {
		resp, err := store.Get(ctx, tc.name)

		// sorting by id to avoid intermittent test failures as order is not guaranteed.
		if len(resp) > 1 {
			sort.Slice(resp, func(i, j int) bool {
				return resp[i].ID < resp[j].ID
			})
		}

		assert.Equal(t, nil, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestCustomer_GetByID(t *testing.T) {
	tests := []struct {
		desc string
		id   string
		err  error
		resp model.Customer
	}{
		{"get by id success", "1", nil, model.Customer{ID: "1", Name: "Henry", City: "Bangalore"}},
		{"get by id fail", "", errors.EntityNotFound{Entity: "customer", ID: ""}, model.Customer{}},
	}

	store := New(index)
	ctx := initializeTests(t)

	for i, tc := range tests {
		resp, err := store.GetByID(ctx, tc.id)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestCustomer_Create(t *testing.T) {
	input := model.Customer{ID: "4", Name: "Elon", City: "Chandigarh"}
	expResp := model.Customer{ID: "4", Name: "Elon", City: "Chandigarh"}

	store := New(index)
	ctx := initializeTests(t)

	resp, err := store.Create(ctx, input)

	assert.Equal(t, nil, err)

	assert.Equal(t, expResp, resp)
}

func TestCustomer_CreateInvalid(t *testing.T) {
	tests := []struct {
		desc  string
		input model.Customer
	}{
		{"create invalid case ", model.Customer{}},
		{"create invalid id case", model.Customer{ID: "", Name: "Musk", City: "NY"}},
	}

	for i, tc := range tests {
		store, ctx := initializeElasticsearchClient()
		resp, err := store.Create(ctx, tc.input)

		assert.IsType(t, err, errors.EntityNotFound{Entity: "customers", ID: ""}, "TEST[%d], failed", i)

		assert.Equal(t, model.Customer{}, resp, "TEST[%d], failed", i)
	}
}

func TestCustomer_Update(t *testing.T) {
	tests := []struct {
		desc  string
		id    string
		input model.Customer
		err   error
		resp  model.Customer
	}{
		{"update existent entity", "4", model.Customer{ID: "4", Name: "Elon", City: "Bangalore"}, nil,
			model.Customer{ID: "4", Name: "Elon", City: "Bangalore"}},
		{"update non existent entity", "444", model.Customer{ID: "444", Name: "Musk", City: "Bangalore"}, nil,
			model.Customer{ID: "444", Name: "Musk", City: "Bangalore"}},
	}

	for i, tc := range tests {
		store := New(index)
		ctx := initializeTests(t)

		resp, err := store.Update(ctx, tc.input, tc.id)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestCustomer_UpdateInvalid(t *testing.T) {
	expResp := model.Customer{}
	expError := errors.EntityNotFound{Entity: "customer"}

	store, ctx := initializeElasticsearchClient()
	resp, err := store.Update(ctx, model.Customer{ID: "", Name: "Musk", City: "Bangalore"}, "")

	assert.IsType(t, err, expError, "TEST failed")

	assert.Equal(t, expResp, resp, "TEST failed.\n%s", "Invalid Id ")
}

func TestCustomer_Delete(t *testing.T) {
	tests := []struct {
		desc string
		id   string
		err  error
	}{
		{"existent entity", "1", nil},
		{"non existent entity", "123", errors.EntityNotFound{Entity: "customer", ID: "123"}},
		{"Invalid ID", "", errors.EntityNotFound{Entity: "customer"}},
	}
	for i, tc := range tests {
		store := New(index)
		ctx := initializeTests(t)

		err := store.Delete(ctx, tc.id)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}
