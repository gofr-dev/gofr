package migration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
)

var errTopic = errors.New("error topic")

func Test_pubsubDS_Methods(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPubSub := NewMockPubSub(ctrl)
	ds := pubsubDS{client: mockPubSub}

	ctx := t.Context()

	t.Run("CreateTopic", func(t *testing.T) {
		mockPubSub.EXPECT().CreateTopic(ctx, "test").Return(nil)
		require.NoError(t, ds.CreateTopic(ctx, "test"))

		mockPubSub.EXPECT().CreateTopic(ctx, "test").Return(errTopic)
		assert.Equal(t, errTopic, ds.CreateTopic(ctx, "test"))
	})

	t.Run("DeleteTopic", func(t *testing.T) {
		mockPubSub.EXPECT().DeleteTopic(ctx, "test").Return(nil)
		require.NoError(t, ds.DeleteTopic(ctx, "test"))

		mockPubSub.EXPECT().DeleteTopic(ctx, "test").Return(errTopic)
		assert.Equal(t, errTopic, ds.DeleteTopic(ctx, "test"))
	})

	t.Run("Query", func(t *testing.T) {
		mockPubSub.EXPECT().Query(ctx, "query", "arg1").Return([]byte("result"), nil)
		res, err := ds.Query(ctx, "query", "arg1")
		require.NoError(t, err)
		assert.Equal(t, []byte("result"), res)

		mockPubSub.EXPECT().Query(ctx, "query").Return(nil, errTopic)
		_, err = ds.Query(ctx, "query")
		assert.Equal(t, errTopic, err)
	})
}

func Test_pubsubDS_apply(t *testing.T) {
	c, _ := container.NewMockContainer(t)
	ds := &Datasource{}

	p := pubsubDS{client: c.PubSub}

	// apply should return a pubsubMigrator that wraps the passed migrator
	result := p.apply(ds)

	pm, ok := result.(pubsubMigrator)
	assert.True(t, ok, "result should be a pubsubMigrator")
	assert.Equal(t, ds, pm.migrator, "pubsubMigrator should wrap the passed migrator")
}

func Test_pubsubMigrator_Delegation(t *testing.T) {
	c, _ := container.NewMockContainer(t)
	ds := &Datasource{}
	p := pubsubMigrator{PubSub: pubsubDS{client: c.PubSub}, migrator: ds}

	// All these methods should delegate to the base migrator (ds) without error
	require.NoError(t, p.checkAndCreateMigrationTable(c))

	v, err := p.getLastMigration(c)
	require.NoError(t, err)
	assert.Equal(t, int64(0), v)

	assert.NotNil(t, p.beginTransaction(c))
	require.NoError(t, p.commitMigration(c, transactionData{}))

	// Should not panic
	p.rollback(c, transactionData{})
	require.NoError(t, p.lock(context.TODO(), nil, c, "owner"))
	require.NoError(t, p.unlock(c, "owner"))

	assert.Equal(t, "PubSub", p.name())
}

func Test_PubSub_GhostDataConflict(t *testing.T) {
	// Setup miniredis
	s := miniredis.RunT(t)

	// Setup Container with Redis and PubSub pointing to the same miniredis using built-in MockConfig
	conf := config.NewMockConfig(map[string]string{
		"APP_NAME":          "integration-test",
		"REDIS_HOST":        s.Host(),
		"REDIS_PORT":        s.Port(),
		"REDIS_DB":          "0",
		"PUBSUB_BACKEND":    "REDIS",
		"REDIS_PUBSUB_DB":   "1",
		"REDIS_PUBSUB_MODE": "streams",
	})

	c := container.NewContainer(conf)
	defer c.Close()

	// Seed Ghost Data in PubSub DB (DB 1)
	client := redis.NewClient(&redis.Options{Addr: s.Addr(), DB: 1})
	err := client.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "gofr_migrations",
		Values: map[string]any{
			"payload": `{"version":20260101000000,"method":"UP","start_time":1700000000,"duration":100}`,
		},
	}).Err()
	require.NoError(t, err)

	// Run Migration Check
	base := &Datasource{Redis: c.Redis}
	rm := redisMigrator{Redis: c.Redis, migrator: base}
	ps := pubsubDS{client: c.PubSub}
	pm := ps.apply(rm)

	version, err := pm.getLastMigration(c)
	require.NoError(t, err)

	assert.Equal(t, int64(0), version, "Migration version should be 0, ignoring ghost data in PubSub")
}

func Test_PubSub_NoEntryAdded(t *testing.T) {
	s := miniredis.RunT(t)

	conf := config.NewMockConfig(map[string]string{
		"REDIS_HOST":      s.Host(),
		"REDIS_PORT":      s.Port(),
		"PUBSUB_BACKEND":  "REDIS",
		"REDIS_PUBSUB_DB": "1",
	})

	c := container.NewContainer(conf)
	defer c.Close()

	base := &Datasource{Redis: c.Redis}
	rm := redisMigrator{Redis: c.Redis, migrator: base}
	ps := pubsubDS{client: c.PubSub}
	pm := ps.apply(rm)

	data := pm.beginTransaction(c)
	data.MigrationNumber = 20240304
	data.StartTime = time.Now()

	err := pm.commitMigration(c, data)
	require.NoError(t, err)

	// Check if entry was added to DB 1 (it should NOT be)
	client := redis.NewClient(&redis.Options{Addr: s.Addr(), DB: 1})
	count, err := client.XLen(context.Background(), "gofr_migrations").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "No migration entry should be added to PubSub backend")

	// Check if entry was added to DB 0
	client0 := redis.NewClient(&redis.Options{Addr: s.Addr(), DB: 0})
	exists, err := client0.HExists(context.Background(), "gofr_migrations", "20240304").Result()
	require.NoError(t, err)
	assert.True(t, exists, "Migration entry should be added to primary Redis DB")
}
