package gofr

import (
	"testing"

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
	})
}

func TestApp_AddClickhouse(t *testing.T) {
	t.Run("Adding Clickhouse", func(t *testing.T) {
		app := New()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := container.NewMockClickhouseProvider(ctrl)

		mock.EXPECT().UseLogger(gomock.Any())
		mock.EXPECT().UseMetrics(gomock.Any())
		mock.EXPECT().Connect()

		app.AddClickhouse(mock)
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
	})
}
