package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

func TestGetSetDelete(t *testing.T) {
	app := gofr.New()
	c := gofr.NewContext(nil, nil, app)
	c.Context = context.Background()

	// initializing the seeder
	seeder := datastore.NewSeeder(&app.DataStore, "../db")
	seeder.RefreshRedis(t, "store")

	testSet(t, c)
	testGet(t, c)
	testDelete(t, c)
	testSetWithError(t, app, c)
}

func testSetWithError(t *testing.T, app *gofr.Gofr, c *gofr.Context) {
	app.Redis.Close()

	expected := "redis: client is closed"

	store := New()
	resp := store.Set(c, "key", "value", 0)

	assert.Equal(t, expected, resp.Error())
}

func testSet(t *testing.T, c *gofr.Context) {
	store := New()

	err := store.Set(c, "someKey123", "someValue123", 0)
	if err != nil {
		t.Errorf("FAILED, Expected no error, Got: %v", err)
	}
}

func testGet(t *testing.T, c *gofr.Context) {
	tests := []struct {
		desc string
		key  string
		resp string
		err  error
	}{
		{"get success", "someKey123", "someValue123", nil},
		{"get fail", "someKey", "", errors.DB{}},
	}

	for i, tc := range tests {
		store := New()
		resp, err := store.Get(c, tc.key)

		assert.Equal(t, tc.resp, resp, "TEST[%d], failed.\n%s", i, tc.desc)

		assert.IsType(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func testDelete(t *testing.T, c *gofr.Context) {
	tests := []struct {
		desc string
		key  string
		err  error
	}{
		{"delete success", "someKey123", nil},
	}

	for i, tc := range tests {
		store := New()
		err := store.Delete(c, tc.key)

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func TestNew(*testing.T) {
	_ = New()
}
