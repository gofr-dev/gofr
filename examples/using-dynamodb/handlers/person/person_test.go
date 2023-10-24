package person

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/examples/using-dynamodb/models"
	"gofr.dev/examples/using-dynamodb/stores"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
)

func initializeTest(t *testing.T, method, url string, body []byte) (*stores.MockPerson, handler, *gofr.Context) {
	mockStore := stores.NewMockPerson(gomock.NewController(t))
	h := New(mockStore)

	req := httptest.NewRequest(method, url, bytes.NewBuffer(body))
	r := request.NewHTTPRequest(req)

	app := gofr.New()
	ctx := gofr.NewContext(nil, r, app)

	return mockStore, h, ctx
}

func TestGetByID(t *testing.T) {
	tests := []struct {
		desc      string
		id        string
		storeResp interface{}
		resp      interface{}
		err       error
	}{
		{"get success", "1", models.Person{ID: "1", Name: "gofr", Email: "gofr@gmail.com"},
			models.Person{ID: "1", Name: "gofr", Email: "gofr@gmail.com"}, nil},
		{"get fail", "2", models.Person{}, nil, errors.DB{}},
	}

	for i, tc := range tests {
		store, h, ctx := initializeTest(t, http.MethodGet, "/person"+tc.id, nil)
		ctx.SetPathParams(map[string]string{
			"id": tc.id,
		})

		store.EXPECT().Get(ctx, tc.id).Return(tc.storeResp, tc.err)

		resp, err := h.GetByID(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestDelete(t *testing.T) {
	store, h, ctx := initializeTest(t, http.MethodDelete, "/person", nil)
	ctx.SetPathParams(map[string]string{
		"id": "1",
	})

	id := ctx.PathParam("id")

	store.EXPECT().Delete(ctx, id).Return(nil)

	_, err := h.Delete(ctx)

	assert.Equal(t, nil, err)
}

func TestCreate(t *testing.T) {
	tests := []struct {
		desc string
		body []byte
		err  error
	}{
		{"create success case", []byte(`{"id":"1", "name":  "gofr", "email": "gofr@zopsmart.com"}`), nil},
		{"create fail case", []byte(`{"id":"1", "name":  "gofr", "email": "gofr@zopsmart.com"}`), errors.DB{}},
	}

	for i, tc := range tests {
		store, h, ctx := initializeTest(t, http.MethodPost, "/person", tc.body)

		var p models.Person

		_ = ctx.Bind(&p)

		store.EXPECT().Create(ctx, p).Return(tc.err)

		_, err := h.Create(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestUpdate(t *testing.T) {
	tests := []struct {
		desc string
		body []byte
		err  error
	}{
		{"update success case", []byte(`{"id":"1", "name":  "gofr", "email": "gofr@zopsmart.com"}`), nil},
		{"update fail case", []byte(`{"id":"1", "name":  "gofr", "email": "gofr@zopsmart.com"}`), errors.DB{}},
	}

	for i, tc := range tests {
		store, h, ctx := initializeTest(t, http.MethodPut, "/person", tc.body)
		ctx.SetPathParams(map[string]string{
			"id": "1",
		})

		id := ctx.PathParam("id")

		var p models.Person

		_ = ctx.Bind(&p)

		p.ID = id

		store.EXPECT().Update(ctx, p).Return(tc.err)

		_, err := h.Update(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func Test_BindError(t *testing.T) {
	body := []byte(`{"id": 1, "name":  "gofr", "email": "gofr@zopsmart.com"}`)

	_, h, ctx := initializeTest(t, http.MethodPut, "/person", body)
	ctx.SetPathParams(map[string]string{
		"id": "1",
	})

	var handlers []gofr.Handler

	handlers = append(handlers, h.Update, h.Create)

	for i := range handlers {
		_, err := handlers[i](ctx)
		assert.Error(t, err, "TEST,failed.")
	}
}
