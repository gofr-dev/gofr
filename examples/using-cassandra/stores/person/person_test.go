//go:build !all

package person

import (
	"reflect"
	"testing"

	"gofr.dev/examples/using-cassandra/models"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"

	"github.com/stretchr/testify/assert"
)

func initializeTest(t *testing.T) *gofr.Gofr {
	app := gofr.New()
	// initializing the seeder
	seeder := datastore.NewSeeder(&app.DataStore, "../../db")
	seeder.RefreshCassandra(t, "persons")

	return app
}

func createMap(input []models.Person) map[models.Person]int {
	output := make(map[models.Person]int)

	for _, val := range input {
		if reflect.DeepEqual(val, models.Person{}) {
			output[val]++
		}
	}

	return output
}

func isSubset(supSet, subSet []models.Person) bool {
	set := createMap(supSet)
	subset := createMap(subSet)

	for k := range subset {
		if val, ok := set[k]; !ok || val != subset[k] {
			return false
		}
	}

	return true
}

func Test_CQL_Get(t *testing.T) {
	tests := []struct {
		desc  string
		input models.Person
		resp  []models.Person
		err   error
	}{
		{"get by id", models.Person{ID: 1}, []models.Person{{ID: 1, Name: "Aakash", Age: 25, State: "Bihar"}}, nil},
		{"get by name", models.Person{Name: "Aakash"}, []models.Person{{ID: 1, Name: "Aakash", Age: 25, State: "Bihar"}}, nil},
		{"get by full info", models.Person{Name: "Aakash", ID: 1, State: "Bihar", Age: 25},
			[]models.Person{{ID: 1, Name: "Aakash", Age: 25, State: "Bihar"}}, nil},
		{"get without info", models.Person{}, []models.Person{
			{ID: 1, Name: "Aakash", Age: 25, State: "Bihar"}, {ID: 3, Name: "Kali", Age: 40, State: "karnataka"}}, nil},
		{"get with partial info", models.Person{ID: 9, State: "Bihar"}, nil, nil},
	}

	app := initializeTest(t)
	ctx := gofr.NewContext(nil, nil, app)

	// create person store
	store := New()

	for i, tc := range tests {
		resp := store.Get(ctx, tc.input)

		if !isSubset(resp, tc.resp) {
			t.Errorf("TEST[%v] Failed.\tExpected %v\tGot %v\n%s", i, tc.resp, resp, tc.desc)
		}
	}
}

func TestCreate(t *testing.T) {
	tests := []struct {
		desc  string
		input models.Person
		resp  []models.Person
		err   error
	}{
		{"create with full info", models.Person{ID: 2, Name: "himari", Age: 30, State: "bihar"},
			[]models.Person{{ID: 2, Name: "himari", Age: 30, State: "bihar"}}, nil},
		{"create with partial info", models.Person{ID: 5, State: "bihar"},
			[]models.Person{{ID: 5, Name: "", Age: 0, State: "bihar"}}, nil},
	}

	app := initializeTest(t)
	ctx := gofr.NewContext(nil, nil, app)

	// create person store
	store := New()

	for i, tc := range tests {
		resp, err := store.Create(ctx, tc.input)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestUpdate(t *testing.T) {
	tests := []struct {
		desc  string
		input models.Person
		resp  []models.Person
		err   error
	}{
		{"update by id", models.Person{ID: 3}, []models.Person{{ID: 3, Name: "Kali", Age: 40, State: "karnataka"}}, nil},
		{"update full info", models.Person{ID: 3, Name: "Mahi", Age: 40, State: "Goa"},
			[]models.Person{{ID: 3, Name: "Mahi", Age: 40, State: "Goa"}}, nil},
		{"update partial info", models.Person{ID: 3, Age: 30, State: "Bihar"},
			[]models.Person{{ID: 3, Name: "Mahi", Age: 30, State: "Bihar"}}, nil},
	}

	app := initializeTest(t)
	ctx := gofr.NewContext(nil, nil, app)

	// create person store
	store := New()

	for i, tc := range tests {
		resp, err := store.Update(ctx, tc.input)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		desc  string
		input string
		err   error
	}{
		{"delete success", "3", nil},
		{"delete fail", "", errors.DB{}},
	}

	app := initializeTest(t)
	ctx := gofr.NewContext(nil, nil, app)

	// create person store
	store := New()

	for i, tc := range tests {
		err := store.Delete(ctx, tc.input)

		assert.IsType(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}
