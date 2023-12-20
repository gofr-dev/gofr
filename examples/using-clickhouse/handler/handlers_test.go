package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"gofr.dev/examples/using-clickhouse/models"
	"gofr.dev/examples/using-clickhouse/store"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
)

func initializeHandlerTest(t *testing.T) (*store.MockStore, handler, *gofr.Gofr) {
	ctrl := gomock.NewController(t)

	mockStore := store.NewMockStore(ctrl)
	h := New(mockStore)
	app := gofr.New()

	return mockStore, h, app
}

func TestGet(t *testing.T) {
	uid := uuid.New()

	tests := []struct {
		desc     string
		mockResp []models.User
		resp     interface{}
		err      error
	}{
		{"success case", []models.User{{ID: uid, Name: "stella", Age: "21"}},
			response{Users: []models.User{{ID: uid, Name: "stella", Age: "21"}}}, nil},
		{"error case", nil, nil, errors.Error("error fetching user listing")},
	}

	mockStore, h, app := initializeHandlerTest(t)

	for i, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "/user", http.NoBody)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)

		mockStore.EXPECT().Get(ctx).Return(tc.mockResp, tc.err)

		result, err := h.Get(ctx)

		assert.Equal(t, tc.resp, result, "TEST[%d] failed.Expected:%v. Got:%v", i, tc.resp, result)
		assert.Equal(t, tc.err, err, "TEST[%d] failed.Expected:%v. Got:%v", i, tc.err, err)
	}
}

func TestGetByID(t *testing.T) {
	uid := uuid.MustParse("37387615-aead-4b28-9adc-78c1eb714ca2")

	tests := []struct {
		desc     string
		id       string
		mockResp models.User
		resp     interface{}
		err      error
	}{
		{"success case", "37387615-aead-4b28-9adc-78c1eb714ca2", models.User{ID: uid, Name: "stella", Age: "21"},
			models.User{ID: uid, Name: "stella", Age: "21"}, nil},
		{"error case", "a", models.User{}, nil, errors.InvalidParam{Param: []string{"id"}}},
		{"error from store", "37387615-aead-4b28-9adc-78c1eb714ca2", models.User{}, nil, errors.DB{Err: errors.Error("db error")}},
	}

	mockStore, h, app := initializeHandlerTest(t)

	for i, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "/user/{id}", http.NoBody)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)

		ctx.SetPathParams(map[string]string{"id": tc.id})

		mockStore.EXPECT().GetByID(ctx, uid).Return(tc.mockResp, tc.err).MaxTimes(1)

		result, err := h.GetByID(ctx)

		assert.Equal(t, tc.resp, result, "TEST[%d] failed.Expected:%v. Got:%v", i, tc.resp, result)
		assert.Equal(t, tc.err, err, "TEST[%d] failed.Expected:%v. Got:%v", i, tc.err, err)
	}
}

func Test_Create(t *testing.T) {
	mockStore, h, app := initializeHandlerTest(t)

	var user models.User

	uid := uuid.MustParse("37387615-aead-4b28-9adc-78c1eb714ca2")

	tests := []struct {
		desc     string
		body     []byte
		mockResp models.User
		resp     interface{}
		err      error
	}{
		{"success", []byte(`{"name":"stella","age":"21"}`), models.User{ID: uid, Name: "stella", Age: "21"},
			models.User{ID: uid, Name: "stella", Age: "21"}, nil},
		{"error from store layer", []byte(`{"name":"stella","age":"21"}`), models.User{}, nil, errors.DB{Err: errors.Error("db error")}},
		{"invalid body", []byte(`{`), models.User{}, nil, errors.InvalidParam{Param: []string{"body"}}},
	}

	for i, tc := range tests {
		req := httptest.NewRequest(http.MethodPost, "/user", bytes.NewReader(tc.body))
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)

		_ = json.Unmarshal(tc.body, &user)

		mockStore.EXPECT().Create(ctx, user).Return(tc.mockResp, tc.err).MaxTimes(1)

		res, err := h.Create(ctx)

		assert.Equal(t, tc.resp, res, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}
