package gofr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/file"
)

func TestApp_AddKVStore(t *testing.T) {
	t.Run("Adding KV-Store", func(t *testing.T) {
		app := New()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := container.NewMockKVStoreProvider(ctrl)

		mock.EXPECT().UseLogger(app.Logger())
		mock.EXPECT().UseMetrics(app.Metrics())
		mock.EXPECT().Connect()

		app.AddKVStore(mock)

		assert.Equal(t, mock, app.container.KVStore)
	})
}

func TestApp_AddMongo(t *testing.T) {
	t.Run("Adding MongoDB", func(t *testing.T) {
		app := New()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := container.NewMockMongoProvider(ctrl)

		mock.EXPECT().UseLogger(app.Logger())
		mock.EXPECT().UseMetrics(app.Metrics())
		mock.EXPECT().Connect()

		app.AddMongo(mock)

		assert.Equal(t, mock, app.container.Mongo)
	})
}

func TestApp_AddCassandra(t *testing.T) {
	t.Run("Adding Cassandra", func(t *testing.T) {
		app := New()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := container.NewMockCassandraProvider(ctrl)

		mock.EXPECT().UseLogger(app.Logger())
		mock.EXPECT().UseMetrics(app.Metrics())
		mock.EXPECT().Connect()

		app.AddCassandra(mock)

		assert.Equal(t, mock, app.container.Cassandra)
	})
}

func TestApp_AddClickhouse(t *testing.T) {
	t.Run("Adding Clickhouse", func(t *testing.T) {
		app := New()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := container.NewMockClickhouseProvider(ctrl)

		mock.EXPECT().UseLogger(app.Logger())
		mock.EXPECT().UseMetrics(app.Metrics())
		mock.EXPECT().Connect()

		app.AddClickhouse(mock)

		assert.Equal(t, mock, app.container.Clickhouse)
	})
}

func TestApp_AddFTP(t *testing.T) {
	t.Run("Adding FTP", func(t *testing.T) {
		app := New()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := file.NewMockFileSystemProvider(ctrl)

		mock.EXPECT().UseLogger(app.Logger())
		mock.EXPECT().UseMetrics(app.Metrics())
		mock.EXPECT().Connect()

		app.AddFTP(mock)

		assert.Equal(t, mock, app.container.File)
	})

	t.Run("Adding FTP", func(t *testing.T) {
		app := New()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := file.NewMockFileSystemProvider(ctrl)

		mock.EXPECT().UseLogger(app.Logger())
		mock.EXPECT().UseMetrics(app.Metrics())
		mock.EXPECT().Connect()

		app.AddFileStore(mock)

		assert.Equal(t, mock, app.container.File)
	})
}

func TestApp_AddS3(t *testing.T) {
	t.Run("Adding S3", func(t *testing.T) {
		app := New()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := file.NewMockFileSystemProvider(ctrl)

		mock.EXPECT().UseLogger(app.Logger())
		mock.EXPECT().UseMetrics(app.Metrics())
		mock.EXPECT().Connect()

		app.AddS3(mock)

		assert.Equal(t, mock, app.container.File)
	})
}
