//go:build !skip

package person

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/examples/using-cassandra/models"
	"gofr.dev/examples/using-cassandra/stores"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
)

func initializeHandlerTest(t *testing.T) (*stores.MockPerson, handler, *gofr.Gofr) {
	ctrl := gomock.NewController(t)

	store := stores.NewMockPerson(ctrl)
	h := New(store)
	app := gofr.New()

	return store, h, app
}

func TestPerson_Get(t *testing.T) {
	tests := []struct {
		desc        string
		queryParams string
		resp        []models.Person
		err         error
	}{
		{"get by id", "id=1", []models.Person{{ID: 1, Name: "Aakash", Age: 25, State: "Bihar"}}, nil},
		{"get by name and age", "name=Aakash&age=25", []models.Person{{ID: 1, Name: "Aakash", Age: 25, State: "Bihar"}}, nil},
		{"get without params", "", []models.Person{
			{ID: 1, Name: "Aakash", Age: 25, State: "Bihar"},
			{ID: 2, Name: "himari", Age: 30, State: "bihar"},
		}, nil},
	}

	store, h, app := initializeHandlerTest(t)

	for i, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "/persons?"+tc.queryParams, nil)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)
		filter := models.Person{}
		val := ctx.Params()

		filter.ID, _ = strconv.Atoi(val["id"])
		filter.Name = val["name"]
		filter.Age, _ = strconv.Atoi(val["age"])
		filter.State = val["state"]

		store.EXPECT().Get(ctx, filter).Return(tc.resp)

		resp, err := h.Get(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestPerson_Create_InvalidInsertionIDAndJSONError(t *testing.T) {
	tests := []struct {
		desc          string
		callGet       bool
		input         string
		resp          interface{}
		mockGetOutput []models.Person
		err           error
	}{
		{
			"json marshal error", false, `{"id":    3, "name":  "Kali", "age":   "40", "State": "karnataka"}`,
			nil, nil,
			&json.UnmarshalTypeError{Value: "string", Type: reflect.TypeOf(40), Offset: 43, Struct: "Person", Field: "age"},
		},
		{
			"entity existing error", true, `{"id":    3, "name":  "Kali", "age":   40, "State": "karnataka"}`,
			nil, []models.Person{{ID: 3, Name: "Kali", Age: 40, State: "karnataka"}},
			errors.EntityAlreadyExists{},
		},
	}

	store, h, app := initializeHandlerTest(t)

	for i, tc := range tests {
		in := strings.NewReader(tc.input)
		req := httptest.NewRequest(http.MethodPost, "/dummy", in)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)

		if tc.callGet == true {
			store.EXPECT().Get(ctx, models.Person{ID: 3}).Return(tc.mockGetOutput).AnyTimes()
		}

		resp, err := h.Create(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestPerson_Create(t *testing.T) {
	tests := []struct {
		desc  string
		input string
		resp  interface{}
		err   error
	}{
		{"create success", `{"id":4, "name":"Kali", "age":40, "State":"Karnataka"}`,
			[]models.Person{{ID: 4, Name: "Kali", Age: 40, State: "karnataka"}}, nil},
	}

	store, h, app := initializeHandlerTest(t)

	for i, tc := range tests {
		in := strings.NewReader(tc.input)
		req := httptest.NewRequest(http.MethodPost, "/dummy", in)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)

		store.EXPECT().Get(ctx, models.Person{ID: 4}).Return(nil)
		store.EXPECT().Create(ctx, models.Person{ID: 4, Name: "Kali", Age: 40, State: "Karnataka"}).Return(tc.resp, tc.err)

		resp, err := h.Create(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestPerson_InvalidUpdateIDAndJSONError(t *testing.T) {
	tests := []struct {
		desc    string
		callGet bool
		id      string
		input   string
		err     error
	}{
		{"json marshal error", false, "3", `{ "name":  "Mali", "age":   "40", "State": "AP"}`,
			&json.UnmarshalTypeError{Value: "string", Type: reflect.TypeOf(40), Offset: 32, Struct: "Person", Field: "age"},
		},
		{"update non esistent entity", true, "5",
			`{ "name":  "Mali", "age":   40, "State": "AP"}`, errors.EntityNotFound{Entity: "person", ID: "5"}},
	}

	store, h, app := initializeHandlerTest(t)

	for i, tc := range tests {
		in := strings.NewReader(tc.input)
		req := httptest.NewRequest(http.MethodPut, "/dummy/"+tc.id, in)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)

		ctx.SetPathParams(map[string]string{
			"id": tc.id,
		})

		if tc.callGet == true {
			id, _ := strconv.Atoi(tc.id)

			store.EXPECT().Get(ctx, models.Person{ID: id}).Return(nil)
		}

		_, err := h.Update(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestPerson_Update(t *testing.T) {
	tests := []struct {
		desc          string
		id            string
		input         string
		resp          interface{}
		err           error
		mockGetOutput []models.Person
	}{
		{
			"update complete info", "3", `{ "name":  "Mali", "age":   40, "State": "AP"}`,
			[]models.Person{{ID: 3, Name: "Mali", Age: 40, State: "AP"}},
			nil, []models.Person{{ID: 3, Name: "Kali", Age: 40, State: "karnataka"}},
		},
		{
			"update partial info", "3", `{  "age":   35, "State": "AP"}`,
			[]models.Person{{ID: 3, Name: "Kali", Age: 35, State: "AP"}},
			nil, []models.Person{{ID: 3, Name: "Kali", Age: 40, State: "karnataka"}},
		},
	}

	store, h, app := initializeHandlerTest(t)

	for i, tc := range tests {
		in := strings.NewReader(tc.input)
		req := httptest.NewRequest(http.MethodPut, "/persons/"+tc.id, in)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)
		filter := models.Person{}

		ctx.SetPathParams(map[string]string{
			"id": tc.id,
		})

		_ = ctx.Bind(&filter)
		id, _ := strconv.Atoi(tc.id)
		filter.ID = id

		store.EXPECT().Get(ctx, models.Person{ID: id}).Return(tc.mockGetOutput)
		store.EXPECT().Update(ctx, filter).Return(tc.resp, tc.err)

		resp, err := h.Update(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestPerson_Delete(t *testing.T) {
	tests := []struct {
		desc          string
		callDel       bool
		id            string
		resp          interface{}
		err           error
		mockGetOutput []models.Person
	}{
		{"delete non existent entity", false, "5", nil, errors.EntityNotFound{Entity: "person", ID: "5"}, nil},
		{"delete success", true, "3", nil, nil, []models.Person{{ID: 3, Name: "Kali", Age: 40, State: "karnataka"}}},
	}

	store, h, app := initializeHandlerTest(t)

	for i, tc := range tests {
		req := httptest.NewRequest(http.MethodPut, "/persons/"+tc.id, nil)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)

		ctx.SetPathParams(map[string]string{
			"id": tc.id,
		})

		id, _ := strconv.Atoi(tc.id)

		store.EXPECT().Get(ctx, models.Person{ID: id}).Return(tc.mockGetOutput)

		if tc.callDel == true {
			store.EXPECT().Delete(ctx, ctx.PathParam("id")).Return(tc.err)
		}

		resp, err := h.Delete(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}
