package gofr

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/file"
)

func TestApp_AddKVStore(t *testing.T) {
	t.Run("Adding KV-Store", func(t *testing.T) {
		app := New()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()

		mock := container.NewMockKVStoreProvider(ctrl)

		mock.EXPECT().UseLogger(app.Logger())
		mock.EXPECT().UseMetrics(app.Metrics())
		mock.EXPECT().Connect(ctx)
		mock.EXPECT().UseTracer(otel.GetTracerProvider().Tracer("gofr-badger"))

		err := app.AddKVStore(ctx, mock)
		require.NoError(t, err)

		assert.Equal(t, mock, app.container.KVStore)
	})
}

func TestApp_AddMongo(t *testing.T) {
	t.Run("Adding MongoDB", func(t *testing.T) {
		app := New()

		ctx := context.Background()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := container.NewMockMongoProvider(ctrl)

		mock.EXPECT().UseLogger(app.Logger())
		mock.EXPECT().UseMetrics(app.Metrics())
		mock.EXPECT().UseTracer(gomock.Any())
		mock.EXPECT().Connect(ctx)

		err := app.AddMongo(ctx, mock)
		require.NoError(t, err)

		assert.Equal(t, mock, app.container.Mongo)
	})
}

func TestApp_AddCassandra(t *testing.T) {
	t.Run("Adding Cassandra", func(t *testing.T) {
		app := New()

		ctx := context.Background()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := container.NewMockCassandraProvider(ctrl)

		mock.EXPECT().UseLogger(app.Logger())
		mock.EXPECT().UseMetrics(app.Metrics())
		mock.EXPECT().UseTracer(otel.GetTracerProvider().Tracer("gofr-cassandra"))
		mock.EXPECT().Connect(ctx)

		err := app.AddCassandra(ctx, mock)
		require.NoError(t, err)

		assert.Equal(t, mock, app.container.Cassandra)
	})
}

func TestApp_AddClickhouse(t *testing.T) {
	t.Run("Adding Clickhouse", func(t *testing.T) {
		app := New()

		ctx := context.Background()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := container.NewMockClickhouseProvider(ctrl)

		mock.EXPECT().UseLogger(app.Logger())
		mock.EXPECT().UseMetrics(app.Metrics())
		mock.EXPECT().UseTracer(otel.GetTracerProvider().Tracer("gofr-clickhouse"))
		mock.EXPECT().Connect(ctx)

		err := app.AddClickhouse(ctx, mock)
		require.NoError(t, err)

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

		app.AddFileStore(mock)

		assert.Equal(t, mock, app.container.File)
	})
}

func TestApp_AddOpenTSDB(t *testing.T) {
	t.Run("Adding OpenTSDB", func(t *testing.T) {
		app := New()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := container.NewMockOpenTSDBProvider(ctrl)

		mock.EXPECT().UseLogger(app.Logger())
		mock.EXPECT().UseMetrics(app.Metrics())
		mock.EXPECT().UseTracer(gomock.Any())
		mock.EXPECT().Connect()

		app.AddOpenTSDB(mock)

		assert.Equal(t, mock, app.container.OpenTSDB)
	})
}
