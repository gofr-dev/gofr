//go:build !all

package shop

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"gofr.dev/examples/using-ycql/models"
	"gofr.dev/examples/using-ycql/stores"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func initializeHandlerTest(t *testing.T) (*stores.MockShop, handler, *gofr.Gofr) {
	ctrl := gomock.NewController(t)

	store := stores.NewMockShop(ctrl)
	h := New(store)
	app := gofr.New()

	return store, h, app
}

func Test_YCQL_Get(t *testing.T) {
	tests := []struct {
		desc        string
		queryParams string
		resp        []models.Shop
		err         error
	}{
		{"Get by id", "id=1", []models.Shop{{ID: 1, Name: "PhoenixMall", Location: "Gaya", State: "Bihar"}}, nil},
		{"Gey by name&location", "name=PhoenixMall&location=Gaya",
			[]models.Shop{{ID: 1, Name: "PhoenixMall", Location: "Gaya", State: "Bihar"}}, nil},
		{"Get without query params", "", []models.Shop{
			{ID: 1, Name: "PhoenixMall", Location: "Gaya", State: "Bihar"},
			{ID: 2, Name: "GarudaMall", Location: "Dhanbad", State: "bihar"},
		}, nil},
	}

	store, h, app := initializeHandlerTest(t)

	for i, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "/shop?"+tc.queryParams, nil)
		r := request.NewHTTPRequest(req)
		context := gofr.NewContext(nil, r, app)
		params := context.Params()

		var shop = models.Shop{Name: params["name"], Location: params["location"], State: params["stats"]}
		shop.ID, _ = strconv.Atoi(params["id"])

		store.EXPECT().Get(context, shop).Return(tc.resp)

		resp, err := h.Get(context)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func Test_YCQL_Create(t *testing.T) {
	store, h, app := initializeHandlerTest(t)

	tests := []struct {
		desc     string
		input    string
		resp     interface{}
		mockResp []models.Shop
		err      error
	}{
		{
			"create success", `{"id":4, "name": "UBCity", "location":"HSR", "State":"karnataka"}`,
			[]models.Shop{{ID: 4, Name: "UBCity", Location: "HSR", State: "karnataka"}}, nil, nil,
		},
		{
			"entity exists", `{"id": 3, "name": "UBCity", "location":"Bangalore", "state":"karnataka"}`,
			nil, []models.Shop{{ID: 3, Name: "UBCity", Location: "HSR", State: "karnataka"}}, errors.EntityAlreadyExists{},
		},
		{
			"unmarshal error", `{"id":"3", "name":"UBCity", "location":"Bangalore", "state":"karnataka"}`, nil,
			nil, &json.UnmarshalTypeError{Value: "string", Type: reflect.TypeOf(40), Offset: 9, Struct: "Shop", Field: "id"},
		},
	}

	for i, tc := range tests {
		in := strings.NewReader(tc.input)
		req := httptest.NewRequest(http.MethodPost, "/dummy", in)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)

		var shop models.Shop

		_ = ctx.Bind(&shop)

		store.EXPECT().Get(ctx, models.Shop{ID: shop.ID}).Return(tc.mockResp).MaxTimes(2)
		store.EXPECT().Create(ctx, shop).Return(tc.resp, tc.err).MaxTimes(1)

		resp, err := h.Create(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func Test_YCQL_Update(t *testing.T) {
	store, h, app := initializeHandlerTest(t)

	tests := []struct {
		desc  string
		id    string
		input string
		resp  interface{}
		err   error
	}{
		{
			"update name,location and state", "3", `{ "name":  "SelectCityWalk", "location":  "tirupati", "State": "AndhraPradesh"}`,
			[]models.Shop{{ID: 3, Name: "SelectCityWalk", Location: "tirupati", State: "AndhraPradesh"}}, nil,
		},
		{
			"udpate location,state", "3", `{  "location":"Dhanbad"  , "state": "Jharkhand"}`,
			[]models.Shop{{ID: 3, Name: "SelectCityWalk", Location: "Dhanbad", State: "Jharkhand"}}, nil,
		},
		{
			"update non existent entity", "5", `{ "name":  "Mali", "age":   40, "State": "AP"}`, nil,
			errors.EntityNotFound{Entity: "shop", ID: "5"},
		},
		{
			"unmarshall error", "3", `{ "name":  "SkyWalkMall", "location":30, "state": "karnataka"}`, nil,
			&json.UnmarshalTypeError{Value: "number", Type: reflect.TypeOf("30"), Offset: 39, Struct: "Shop", Field: "location"},
		},
	}

	for i, tc := range tests {
		in := strings.NewReader(tc.input)
		req := httptest.NewRequest(http.MethodPut, "/shop/"+tc.id, in)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)

		ctx.SetPathParams(map[string]string{
			"id": tc.id,
		})

		var shop models.Shop

		_ = ctx.Bind(&shop)
		shop.ID, _ = strconv.Atoi(ctx.PathParam("id"))

		store.EXPECT().Get(ctx, models.Shop{ID: shop.ID}).Return(tc.resp).MaxTimes(3)
		store.EXPECT().Update(ctx, shop).Return(tc.resp, tc.err).MaxTimes(2)

		resp, err := h.Update(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func Test_YCQL_Delete(t *testing.T) {
	store, h, app := initializeHandlerTest(t)

	tests := []struct {
		desc     string
		id       string
		resp     interface{}
		mockResp []models.Shop
		err      error
	}{
		{"delete non existent item fail", "5", nil, nil, errors.EntityNotFound{Entity: "shop", ID: "5"}},
		{"delete existent item success", "3", nil, []models.Shop{{ID: 3, Name: "Kali", Location: "HSR", State: "karnataka"}}, nil},
	}

	for i, tc := range tests {
		req := httptest.NewRequest(http.MethodDelete, "/shop/"+tc.id, nil)
		r := request.NewHTTPRequest(req)
		ctx := gofr.NewContext(nil, r, app)

		ctx.SetPathParams(map[string]string{
			"id": tc.id,
		})

		var shop models.Shop

		shop.ID, _ = strconv.Atoi(ctx.PathParam("id"))

		id := ctx.PathParam("id")

		store.EXPECT().Get(ctx, shop).Return(tc.mockResp)
		store.EXPECT().Delete(ctx, id).Return(nil).MaxTimes(1)

		resp, err := h.Delete(ctx)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}
