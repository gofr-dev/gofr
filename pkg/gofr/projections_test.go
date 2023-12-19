package gofr

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/request"
)

type successCase struct {
	target              string
	resource            string
	expectedProjections ProjectionMapType
}

type errorCase struct {
	target   string
	resource string
}

func TestContext_Projections_success(t *testing.T) {
	testCases := getSuccessCases()

	g := Gofr{
		ResourceMap: map[string][]string{
			"products": {"inventory", "images", "brand", "prices"},
			"brand":    {"certifications"},
		},
		// compact shape is checked by default. Need not have to specify again while declaring ResourceCustomShapes
		ResourceCustomShapes: map[string][]string{
			"products":  {"full", "medium"},
			"inventory": {"full"},
			"images":    {"full"},
			"brand":     {"full"},
			"prices":    {"full"},
		},
	}

	for i, tc := range testCases {
		r := httptest.NewRequest(http.MethodGet, tc.target, http.NoBody)
		req := request.NewHTTPRequest(r)

		c := NewContext(nil, req, &g)

		value, err := c.Projections(tc.resource)

		if !assert.Equal(t, tc.expectedProjections, value) {
			t.Errorf("Incorrect value for projection case %v. Expected: %s\tGot %s", i+1, tc.expectedProjections, value)
		}

		if err != nil {
			t.Errorf("Incorrect error for projection case %v. Expected: %v\tGot %v", i+1, nil, err)
		}
	}
}

func TestContext_Projections_errors(t *testing.T) {
	testcases := getErrorsCases()

	g := Gofr{
		ResourceMap: map[string][]string{
			"products": {"inventory", "images", "brand", "prices"},
			"brand":    {"certifications"},
			"recipes":  {"brand"},
		},
		// compact shape is checked by default. Need not have to specify again while declaring ResourceCustomShapes
		ResourceCustomShapes: map[string][]string{
			"inventory": {"full"},
			"images":    {"full"},
			"brand":     {"full"},
			"prices":    {"full"},
			"recipes":   {"full"},
		},
	}

	for i, tc := range testcases {
		r := httptest.NewRequest(http.MethodGet, tc.target, http.NoBody)
		req := request.NewHTTPRequest(r)

		c := NewContext(nil, req, &g)
		value, err := c.Projections(tc.resource)

		if !assert.Equal(t, ProjectionMapType(nil), value) {
			t.Errorf("Incorrect value for projection case %v. Expected: %v\tGot %s", i+1, ProjectionMapType(nil), value)
		}

		if !assert.Equal(t, invalidProjection, err) {
			t.Errorf("Incorrect error for projection case %v. Expected: %v\tGot %v", i+1, invalidProjection, err)
		}
	}
}

// Separate function is required for getting testcases as the Test function length was increasing
func getSuccessCases() []successCase {
	return []successCase{
		{"/test?projections=products[inventory.full,brand[certifications.compact]]",
			"products", ProjectionMapType{
				"inventory":      "full",
				"certifications": "compact",
				"products":       "compact",
			},
		},
		{"/test?projections=products.compact,products[inventory.full,brand[certifications.compact]]",
			"products", ProjectionMapType{
				"certifications": "compact",
				"inventory":      "full",
				"products":       "compact",
			},
		},
		{"/test?projections=brand.compact",
			"brand", ProjectionMapType{
				"brand": "compact",
			},
		},
		{"/test?projections=brand.full",
			"brand", ProjectionMapType{
				"certifications": "compact",
				"brand":          "full",
			},
		},
		{"/test?projections=products.full",
			"products", ProjectionMapType{
				"certifications": "compact",
				"images":         "full",
				"inventory":      "full",
				"brand":          "full",
				"prices":         "full",
				"products":       "full",
			},
		},
		{"/test?projections=products.full",
			"products", ProjectionMapType{
				"certifications": "compact",
				"images":         "full",
				"inventory":      "full",
				"brand":          "full",
				"prices":         "full",
				"products":       "full",
			},
		},
		{"/test/brand-core/123",
			"brand", ProjectionMapType{
				"brand": "compact",
			},
		},
		{"/test?projections=products.compact,products[brand.full]",
			"products", ProjectionMapType{
				"products":       "compact",
				"brand":          "full",
				"certifications": "compact",
			},
		},
		{"/test?projections=products[brand.full],products.compact",
			"products", ProjectionMapType{
				"products":       "compact",
				"brand":          "full",
				"certifications": "compact",
			},
		},
		{"/test?projections=products[brand.compact,brand[certifications.compact]],products.medium",
			"products", ProjectionMapType{
				"products":       "medium",
				"brand":          "compact",
				"certifications": "compact",
			},
		},
		{"/test?projections=products.medium,products[brand.compact,brand[certifications.compact]]",
			"products", ProjectionMapType{
				"products":       "medium",
				"brand":          "compact",
				"certifications": "compact",
			},
		},

		{"/test?projections=products[brand.compact,brand[certifications.compact]],products.medium,products[inventory.full]",
			"products", ProjectionMapType{
				"products":       "medium",
				"brand":          "compact",
				"certifications": "compact",
				"inventory":      "full",
			},
		},
	}
}

func getErrorsCases() []errorCase {
	return []errorCase{
		{"/test?projections=products[inventory.full,brand[certifications.compact]]", "products"},
		{"/test?projections=products[products.compact,inventory.compact,brand[certifications.compact]]", "products"},
		{"/test?projections=products[products.compact]", "products"},
		{"/test?projections=recipes.full,recipes.compact,recipes[brand.compact]", "recipes"},
		{"/test?projections=recipes[brand.compact]", "brand"},
		{"/test?projections=random.full", "random"},
		{"/test?projections=product.full", "products"},
		{"/test?projections=recipes.full,recipes.,recipes[.compact]", "recipes"},
		{"/test?projections=recipes.", "recipes"},
		{"/test?projections=.full,recipes.full,recipes[.compact]", "recipes"},
		{"/test?projections=products.compact,brand.compact", "products"},
		{"/test?projections=brand.compact,products.compact", "products"},
		{"/test?projections=products[brand.full,brand[certifications.full]],products.compact", "products"},
		{"/test?projections=products[brand.full,certifications.full],products.compact", "products"},
		{"/test?projections=products.compact,products[xyz[brand[certifications.compact]]]", "products"},
		{"/test?projections=products.compact,products[brand[inventory.compact]]]", "products"},
		{"/test?projections=products[xyz[certifications.compact]],products.medium", "products"},
		{"/test?projections=products[xyz[certifications.compact]],products.compact,products[inventory.full]", "products"},
	}
}
