package couchbase

import (
	"context"
	"testing"
	"time"

	"github.com/couchbase/gocb/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestClient_New(t *testing.T) {
	client := New(&Config{
		Host:              "localhost",
		User:              "Administrator",
		Password:          "password",
		Bucket:            "gofr",
		ConnectionTimeout: time.Second * 5,
	})

	assert.NotNil(t, client)
}

func TestClient_Upsert(t *testing.T) {
	var (
		ctrl       = gomock.NewController(t)
		cluster    = NewMockclusterProvider(ctrl)
		bucket     = NewMockbucketProvider(ctrl)
		collection = NewMockcollectionProvider(ctrl)
		logger     = NewMockLogger(ctrl)
		metrics    = NewMockMetrics(ctrl)
	)

	client := Client{
		cluster: cluster,
		bucket:  bucket,
		config:  &Config{},
		logger:  logger,
		metrics: metrics,
	}

	t.Run("upsert", func(t *testing.T) {
		wantResult := &gocb.MutationResult{Result: gocb.Result{}}

		bucket.EXPECT().DefaultCollection().Return(collection)
		collection.EXPECT().Upsert("test bucket", map[string]string{"key": "value"}, gomock.Any()).
			Return(wantResult, nil)

		var result *gocb.MutationResult
		err := client.Upsert(context.Background(), "test bucket", map[string]string{"key": "value"}, &result)

		assert.Nil(t, err)
		assert.Equal(t, wantResult, result)
	})
}
