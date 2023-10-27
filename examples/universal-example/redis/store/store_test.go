package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

func initializeTest(t *testing.T) *gofr.Gofr {
	g := gofr.New()

	// initialize the seeder
	seeder := datastore.NewSeeder(&g.DataStore, "../db")
	seeder.RefreshRedis(t, "store")

	return g
}

func TestModel_Get(t *testing.T) {
	tests := []struct {
		key         string
		value       string
		expectedErr error
	}{
		// success
		{"first", "Aman", nil},
		// failure
		{"someKey", "", errors.DB{}},
	}
	for i, tc := range tests {
		g := initializeTest(t)
		c := gofr.NewContext(nil, nil, g)
		c.Context = context.Background()

		gotValue, gotErr := New().Get(c, tc.key)
		if gotErr != tc.expectedErr && tc.expectedErr == nil {
			t.Errorf("TestCase[%v]  \tFAILED, \nExpected: %v\nGot: %v\n", i, tc.expectedErr, gotErr)
		}

		assert.Equal(t, tc.value, gotValue)
	}
}

func TestModel_Set(t *testing.T) {
	k := initializeTest(t)
	c := gofr.NewContext(nil, nil, k)
	c.Context = context.Background()

	err := New().Set(c, "someKey123", "someValue123", 0)
	assert.Equal(t, nil, err)
}

func TestModel_SetWithError(t *testing.T) {
	k := initializeTest(t)
	c := gofr.NewContext(nil, nil, k)
	c.Context = context.Background()

	k.Redis.Close()

	gotErr := New().Set(c, "key", "value", 0).Error()
	expectedErr := "redis: client is closed"

	assert.Equal(t, expectedErr, gotErr)
}
