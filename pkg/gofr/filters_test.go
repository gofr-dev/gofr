package gofr

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/request"
)

func TestContext_Filters(t *testing.T) {
	testCases := []struct {
		target         string
		expectedFilter Filters
		listCheck      string
	}{
		{
			"/test?filter.locationIds=lid38498,lid4390&filter.locationIds=lid98080",
			Filters{
				"locationIds": map[string]string{"eq": "lid38498,lid4390,lid98080"},
			},
			"",
		},
		{"/test?filter.description.like=sprite&filter.price.lte=3.99",
			Filters{
				"description": map[string]string{"like": "sprite"},
				"price":       map[string]string{"lte": "3.99"},
			},
			"",
		},
		{
			"/test?filter.description.like=sprite&filter.description.like=7-up&filter.description.ne=sprites",
			Filters{
				"description": map[string]string{
					"like": "sprite,7-up",
					"ne":   "sprites",
				},
			},
			"description",
		},
	}
	for i, tc := range testCases {
		r := httptest.NewRequest(http.MethodGet, tc.target, http.NoBody)
		req := request.NewHTTPRequest(r)

		c := NewContext(nil, req, nil)
		value := c.Filters()

		if !reflect.DeepEqual(value, tc.expectedFilter) {
			// Checking for equality of slice without order
			if tc.listCheck == "" {
				t.Errorf("Incorrect value for filter test case %v. Expected: %s\tGot %s", i+1, tc.expectedFilter, value)
				break
			}

			if !assert.ElementsMatch(t, value[tc.listCheck], tc.expectedFilter[tc.listCheck]) {
				t.Errorf("Incorrect value for filter test case %v. Expected: %s\tGot %s", i+1, tc.expectedFilter, value)
			}
		}
	}
}
