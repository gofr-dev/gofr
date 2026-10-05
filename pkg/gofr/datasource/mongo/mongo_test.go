package mongo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"
)

func TestClient_Connect(t *testing.T) {
	tests := []struct {
		desc        string
		config      Config
		setupMocks  func(l *MockLogger, m *MockMetrics)
		expURI      string
		expDatabase string
		expDBSet    bool
	}{
		{
			desc:   "invalid config registers metrics and leaves database nil",
			config: Config{Host: "mongo", Database: "test"},
			setupMocks: func(l *MockLogger, m *MockMetrics) {
				m.EXPECT().NewHistogram("app_mongo_stats", gomock.Any(), gomock.Any())
				l.EXPECT().Errorf("error generating MongoDB URI: %v", gomock.Any())
			},
		},
		{
			desc:   "server unreachable registers metrics and keeps the client",
			config: Config{URI: "mongodb://127.0.0.1:1/test", Database: "test", ConnectionTimeout: 200 * time.Millisecond},
			setupMocks: func(l *MockLogger, m *MockMetrics) {
				m.EXPECT().NewHistogram("app_mongo_stats", gomock.Any(), gomock.Any())
				l.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any())
				l.EXPECT().Errorf(gomock.Any(), "127.0.0.1", gomock.Any())
			},
			expURI:      "127.0.0.1",
			expDatabase: "test",
			expDBSet:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			logger := NewMockLogger(ctrl)
			metrics := NewMockMetrics(ctrl)

			tc.setupMocks(logger, metrics)

			client := New(tc.config)
			client.UseLogger(logger)
			client.UseMetrics(metrics)
			client.Connect()

			require.Equal(t, tc.expURI, client.uri)
			require.Equal(t, tc.expDatabase, client.database)
			require.Equal(t, tc.expDBSet, client.Database != nil)
		})
	}
}

func TestGenerateMongoURI(t *testing.T) {
	tests := []struct {
		name          string
		config        Config
		expectedURI   string
		expectedHost  string
		expectedError string
	}{
		{
			name: "Valid Config",
			config: Config{
				User:     "admin",
				Password: "p@##word:",
				Host:     "localhost",
				Port:     27017,
				Database: "mydb",
			},
			expectedURI:   "mongodb://admin:p%2540%2523%2523word%253A@localhost:27017/mydb?authSource=admin",
			expectedHost:  "localhost",
			expectedError: "",
		},
		{
			name: "Valid Config without authentication",
			config: Config{
				Host:     "localhost",
				Port:     27017,
				Database: "mydb",
			},
			expectedURI:   "mongodb://localhost:27017/mydb?authSource=admin",
			expectedHost:  "localhost",
			expectedError: "",
		},
		{
			name: "Predefined URI",
			config: Config{
				URI: "mongodb://admin:password@localhost:27017/mydb?authSource=admin",
			},
			expectedURI:   "mongodb://admin:password@localhost:27017/mydb?authSource=admin",
			expectedHost:  "localhost",
			expectedError: "",
		},
		{
			name: "Empty Host",
			config: Config{
				User:     "admin",
				Password: "password",
				Port:     27017,
				Database: "mydb",
			},
			expectedURI:   "",
			expectedHost:  "",
			expectedError: "missing required field in config: host is empty",
		},
		{
			name: "Invalid Port",
			config: Config{
				User:     "admin",
				Password: "password",
				Host:     "localhost",
				Database: "mydb",
			},
			expectedURI:   "",
			expectedHost:  "",
			expectedError: "missing required field in config: port is empty",
		},
		{
			name: "Empty Database",
			config: Config{
				User:     "admin",
				Password: "password",
				Host:     "localhost",
				Port:     27017,
			},
			expectedURI:   "",
			expectedHost:  "",
			expectedError: "missing required field in config: database is empty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := Client{config: &test.config}
			uri, host, err := generateMongoURI(client.config)

			assert.Equal(t, test.expectedURI, uri, "Unexpected URI")
			assert.Equal(t, test.expectedHost, host, "Unexpected Host")

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError, "Unexpected error message")
			} else {
				assert.NoError(t, err, "Expected no error but got one")
			}
		})
	}
}

func TestGetDBHost(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		expected    string
		expectedErr string
	}{
		{
			name:        "Valid URI with host and port",
			uri:         "mongodb://username:password@hostname:27017/database?authSource=admin",
			expected:    "hostname",
			expectedErr: "",
		},
		{
			name:        "Valid URI with IP address as host",
			uri:         "mongodb://username:password@192.168.1.1:27017/database?authSource=admin",
			expected:    "192.168.1.1",
			expectedErr: "",
		},
		{
			name:        "Invalid URI with no host",
			uri:         "mongodb://username:password@:27017/database?authSource=admin",
			expected:    "",
			expectedErr: "failed to parse host from MongoDB URI",
		},
		{
			name:        "Empty URI",
			uri:         "",
			expected:    "",
			expectedErr: "parse \"\": empty url",
		},
		{
			name:        "Malformed URI",
			uri:         "mongodb:/username:password@hostname:27017/database?authSource=admin",
			expected:    "",
			expectedErr: "failed to parse host from MongoDB URI",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, err := getDBHost(tt.uri)

			assert.Equal(t, tt.expected, host, "Test case: %s", tt.name)

			if tt.expectedErr == "" {
				assert.NoError(t, err, "Test case: %s", tt.name)
			} else {
				assert.EqualError(t, err, tt.expectedErr, "Test case: %s", tt.name)
			}
		})
	}
}

func Test_InsertCommands(t *testing.T) {
	// Create a connected client using the mock database
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metrics := NewMockMetrics(ctrl)
	logger := NewMockLogger(ctrl)

	cl := Client{metrics: metrics, tracer: otel.GetTracerProvider().Tracer("gofr-mongo")}

	metrics.EXPECT().RecordHistogram(context.Background(), "app_mongo_stats", gomock.Any(), "hostname",
		gomock.Any(), "database", gomock.Any(), "type", gomock.Any()).Times(4)

	logger.EXPECT().Debug(gomock.Any()).Times(4)

	cl.logger = logger

	mt.Run("insertOneSuccess", func(mt *mtest.T) {
		cl.Database = mt.DB
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		doc := map[string]any{"name": "Aryan"}

		resp, err := cl.InsertOne(context.Background(), mt.Coll.Name(), doc)

		assert.NotNil(t, resp)
		assert.NoError(t, err)
	})

	mt.Run("insertOneError", func(mt *mtest.T) {
		cl.Database = mt.DB
		mt.AddMockResponses(mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Index:   1,
			Code:    11000,
			Message: "duplicate key error",
		}))

		doc := map[string]any{"name": "Aryan"}

		resp, err := cl.InsertOne(context.Background(), mt.Coll.Name(), doc)

		assert.Nil(t, resp)
		assert.Error(t, err)
	})

	mt.Run("insertManySuccess", func(mt *mtest.T) {
		cl.Database = mt.DB
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		doc := map[string]any{"name": "Aryan"}

		resp, err := cl.InsertMany(context.Background(), mt.Coll.Name(), []any{doc, doc})

		assert.NotNil(t, resp)
		require.NoError(t, err)
	})

	mt.Run("insertManyError", func(mt *mtest.T) {
		cl.Database = mt.DB
		mt.AddMockResponses(mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Index:   1,
			Code:    11000,
			Message: "duplicate key error",
		}))

		doc := map[string]any{"name": "Aryan"}

		resp, err := cl.InsertMany(context.Background(), mt.Coll.Name(), []any{doc, doc})

		assert.Nil(t, resp)
		require.Error(t, err)
	})
}

func Test_CreateCollection(t *testing.T) {
	// Create a connected client using the mock database
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metrics := NewMockMetrics(ctrl)
	logger := NewMockLogger(ctrl)

	cl := Client{metrics: metrics, tracer: otel.GetTracerProvider().Tracer("gofr-mongo")}

	metrics.EXPECT().RecordHistogram(context.Background(), "app_mongo_stats", gomock.Any(), "hostname",
		gomock.Any(), "database", gomock.Any(), "type", gomock.Any())

	logger.EXPECT().Debug(gomock.Any())

	cl.logger = logger

	mt.Run("createCollection", func(mt *mtest.T) {
		cl.Database = mt.DB
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		err := cl.CreateCollection(context.Background(), mt.Coll.Name())

		require.NoError(t, err)
	})
}

func Test_FindMultipleCommands(t *testing.T) {
	// Create a connected client using the mock database
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metrics := NewMockMetrics(ctrl)
	logger := NewMockLogger(ctrl)

	cl := Client{metrics: metrics, tracer: otel.GetTracerProvider().Tracer("gofr-mongo")}

	metrics.EXPECT().RecordHistogram(context.Background(), "app_mongo_stats", gomock.Any(), "hostname",
		gomock.Any(), "database", gomock.Any(), "type", gomock.Any()).Times(3)

	logger.EXPECT().Debug(gomock.Any()).Times(3)

	cl.logger = logger

	mt.Run("FindSuccess", func(mt *mtest.T) {
		cl.Database = mt.DB

		var foundDocuments []any

		id1 := primitive.NewObjectID()

		first := mtest.CreateCursorResponse(1, "foo.bar", mtest.FirstBatch, bson.D{
			{Key: "_id", Value: id1},
			{Key: "name", Value: "john"},
			{Key: "email", Value: "john.doe@test.com"},
		})

		killCursors := mtest.CreateCursorResponse(0, "foo.bar", mtest.NextBatch)
		mt.AddMockResponses(first, killCursors)

		mt.AddMockResponses(first)

		err := cl.Find(context.Background(), mt.Coll.Name(), bson.D{{}}, &foundDocuments)

		assert.NoError(t, err, "Unexpected error during Find operation")
	})

	mt.Run("FindCursorError", func(mt *mtest.T) {
		cl.Database = mt.DB
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		err := cl.Find(context.Background(), mt.Coll.Name(), bson.D{{}}, nil)

		require.ErrorContains(t, err, "database response does not contain a cursor")
	})

	mt.Run("FindCursorParseError", func(mt *mtest.T) {
		cl.Database = mt.DB

		var foundDocuments []any

		id1 := primitive.NewObjectID()

		first := mtest.CreateCursorResponse(1, "foo.bar", mtest.FirstBatch, bson.D{
			{Key: "_id", Value: id1},
			{Key: "name", Value: "john"},
			{Key: "email", Value: "john.doe@test.com"},
		})

		mt.AddMockResponses(first)

		mt.AddMockResponses(first)

		err := cl.Find(context.Background(), mt.Coll.Name(), bson.D{{}}, &foundDocuments)

		require.ErrorContains(t, err, "cursor.nextBatch should be an array but is a BSON invalid")
	})
}

func Test_FindOneCommands(t *testing.T) {
	// Create a connected client using the mock database
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metrics := NewMockMetrics(ctrl)
	logger := NewMockLogger(ctrl)

	cl := Client{metrics: metrics, tracer: otel.GetTracerProvider().Tracer("gofr-mongo")}

	metrics.EXPECT().RecordHistogram(context.Background(), "app_mongo_stats", gomock.Any(), "hostname",
		gomock.Any(), "database", gomock.Any(), "type", gomock.Any()).Times(2)

	logger.EXPECT().Debug(gomock.Any()).Times(2)

	cl.logger = logger

	mt.Run("FindOneSuccess", func(mt *mtest.T) {
		cl.Database = mt.DB

		type user struct {
			ID    primitive.ObjectID
			Name  string
			Email string
		}

		var foundDocuments user

		expectedUser := user{
			ID:    primitive.NewObjectID(),
			Name:  "john",
			Email: "john.doe@test.com",
		}

		mt.AddMockResponses(mtest.CreateCursorResponse(1, "foo.bar", mtest.FirstBatch, bson.D{
			{Key: "_id", Value: expectedUser.ID},
			{Key: "name", Value: expectedUser.Name},
			{Key: "email", Value: expectedUser.Email},
		}))

		err := cl.FindOne(context.Background(), mt.Coll.Name(), bson.D{{}}, &foundDocuments)

		assert.Equal(t, expectedUser.Name, foundDocuments.Name)
		assert.NoError(t, err)
	})

	mt.Run("FindOneError", func(mt *mtest.T) {
		cl.Database = mt.DB

		type user struct {
			ID    primitive.ObjectID
			Name  string
			Email string
		}

		var foundDocuments user

		mt.AddMockResponses(mtest.CreateCursorResponse(1, "foo.bar", mtest.FirstBatch))

		err := cl.FindOne(context.Background(), mt.Coll.Name(), bson.D{{}}, &foundDocuments)

		assert.Error(t, err)
	})
}

func Test_UpdateCommands(t *testing.T) {
	// Create a connected client using the mock database
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metrics := NewMockMetrics(ctrl)
	logger := NewMockLogger(ctrl)

	cl := Client{metrics: metrics, tracer: otel.GetTracerProvider().Tracer("gofr-mongo")}

	metrics.EXPECT().RecordHistogram(context.Background(), "app_mongo_stats", gomock.Any(), "hostname",
		gomock.Any(), "database", gomock.Any(), "type", gomock.Any()).Times(3)

	logger.EXPECT().Debug(gomock.Any()).Times(3)

	cl.logger = logger

	mt.Run("updateByID", func(mt *mtest.T) {
		cl.Database = mt.DB
		mt.AddMockResponses(mtest.CreateSuccessResponse())
		// Create a document to insert

		resp, err := cl.UpdateByID(context.Background(), mt.Coll.Name(), "1", bson.M{"$set": bson.M{"name": "test"}})

		assert.NotNil(t, resp)
		assert.NoError(t, err)
	})

	mt.Run("updateOne", func(mt *mtest.T) {
		cl.Database = mt.DB
		mt.AddMockResponses(mtest.CreateSuccessResponse())
		// Create a document to insert

		err := cl.UpdateOne(context.Background(), mt.Coll.Name(), bson.D{{Key: "name", Value: "test"}},
			bson.M{"$set": bson.M{"name": "testing"}})

		assert.NoError(t, err)
	})

	mt.Run("updateMany", func(mt *mtest.T) {
		cl.Database = mt.DB
		mt.AddMockResponses(mtest.CreateSuccessResponse())
		// Create a document to insert

		_, err := cl.UpdateMany(context.Background(), mt.Coll.Name(), bson.D{{Key: "name", Value: "test"}},
			bson.M{"$set": bson.M{"name": "testing"}})

		assert.NoError(t, err)
	})
}

// On a write error the driver still returns an UpdateResult for UpdateMany (processWriteError
// maps it to rrMany), but returns none for the single-document UpdateOne/UpdateByID. The count
// must survive the first case, and the missing result must not panic the second.
func TestClient_UpdateCountWithWriteError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	logger := NewMockLogger(ctrl)

	metrics.EXPECT().RecordHistogram(gomock.Any(), "app_mongo_stats", gomock.Any(), "hostname",
		gomock.Any(), "database", gomock.Any(), "type", gomock.Any()).AnyTimes()
	logger.EXPECT().Debug(gomock.Any()).AnyTimes()

	partialFailure := bson.D{
		{Key: "ok", Value: 1}, {Key: "n", Value: 3}, {Key: "nModified", Value: 2},
		{Key: "writeErrors", Value: bson.A{bson.D{
			{Key: "index", Value: 2}, {Key: "code", Value: 11000}, {Key: "errmsg", Value: "duplicate key"},
		}}},
	}

	tests := []struct {
		desc     string
		call     func(ctx context.Context, c *Client) (int64, error)
		expCount int64
	}{
		{desc: "UpdateMany keeps the modified count", expCount: 2,
			call: func(ctx context.Context, c *Client) (int64, error) {
				return c.UpdateMany(ctx, "col", bson.M{}, bson.M{"$set": bson.M{"a": 1}})
			}},
		{desc: "UpdateByID gets no result from the driver", expCount: 0,
			call: func(ctx context.Context, c *Client) (int64, error) {
				return c.UpdateByID(ctx, "col", 1, bson.M{"$set": bson.M{"a": 1}})
			}},
	}

	for _, tc := range tests {
		mt.Run(tc.desc, func(mt *mtest.T) {
			cl := &Client{Database: mt.DB, metrics: metrics, logger: logger, tracer: otel.GetTracerProvider().Tracer("gofr-mongo")}
			mt.AddMockResponses(partialFailure)

			count, err := tc.call(mt.Context(), cl)

			var writeErr mongo.WriteException

			require.ErrorAs(mt, err, &writeErr)
			assert.Equal(mt, tc.expCount, count)
		})
	}
}

func Test_CountDocuments(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metrics := NewMockMetrics(ctrl)
	logger := NewMockLogger(ctrl)

	cl := Client{metrics: metrics, tracer: otel.GetTracerProvider().Tracer("gofr-mongo")}

	metrics.EXPECT().RecordHistogram(context.Background(), "app_mongo_stats", gomock.Any(), "hostname",
		gomock.Any(), "database", gomock.Any(), "type", gomock.Any())

	logger.EXPECT().Debug(gomock.Any())

	cl.logger = logger

	mt.Run("countDocuments", func(mt *mtest.T) {
		cl.Database = mt.DB

		mt.AddMockResponses(mtest.CreateSuccessResponse())

		mt.AddMockResponses(mtest.CreateCursorResponse(1, "test.restaurants", mtest.FirstBatch, bson.D{{Key: "n", Value: 1}}))

		// For count to work, mongo needs an index. So we need to create that. Index view should contain a key. Value does not matter
		indexView := mt.Coll.Indexes()
		_, err := indexView.CreateOne(context.Background(), mongo.IndexModel{
			Keys: bson.D{{Key: "x", Value: 1}},
		})

		require.NoError(mt, err, "CreateOne error for index: %v", err)

		resp, err := cl.CountDocuments(context.Background(), mt.Coll.Name(), bson.D{{Key: "name", Value: "test"}})

		assert.Equal(t, int64(1), resp)
		assert.NoError(t, err)
	})
}

func Test_DeleteCommands(t *testing.T) {
	// Create a connected client using the mock database
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metrics := NewMockMetrics(ctrl)
	logger := NewMockLogger(ctrl)

	cl := Client{metrics: metrics, tracer: otel.GetTracerProvider().Tracer("gofr-mongo")}

	metrics.EXPECT().RecordHistogram(context.Background(), "app_mongo_stats", gomock.Any(), "hostname",
		gomock.Any(), "database", gomock.Any(), "type", gomock.Any()).Times(4)

	logger.EXPECT().Debug(gomock.Any()).Times(4)

	cl.logger = logger

	mt.Run("DeleteOne", func(mt *mtest.T) {
		cl.Database = mt.DB
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		resp, err := cl.DeleteOne(context.Background(), mt.Coll.Name(), bson.D{{}})

		assert.Equal(t, int64(0), resp)
		assert.NoError(t, err)
	})

	mt.Run("DeleteOneError", func(mt *mtest.T) {
		cl.Database = mt.DB
		mt.AddMockResponses(mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Index:   1,
			Code:    11000,
			Message: "duplicate key error",
		}))

		resp, err := cl.DeleteOne(context.Background(), mt.Coll.Name(), bson.D{{}})

		assert.Equal(t, int64(0), resp)
		assert.Error(t, err)
	})

	mt.Run("DeleteMany", func(mt *mtest.T) {
		cl.Database = mt.DB
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		resp, err := cl.DeleteMany(context.Background(), mt.Coll.Name(), bson.D{{}})

		assert.Equal(t, int64(0), resp)
		assert.NoError(t, err)
	})

	mt.Run("DeleteManyError", func(mt *mtest.T) {
		cl.Database = mt.DB
		mt.AddMockResponses(mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Index:   1,
			Code:    11000,
			Message: "duplicate key error",
		}))

		resp, err := cl.DeleteMany(context.Background(), mt.Coll.Name(), bson.D{{}})

		assert.Equal(t, int64(0), resp)
		assert.Error(t, err)
	})
}

func Test_Drop(t *testing.T) {
	// Create a connected client using the mock database
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metrics := NewMockMetrics(ctrl)
	logger := NewMockLogger(ctrl)

	cl := Client{metrics: metrics, tracer: otel.GetTracerProvider().Tracer("gofr-mongo")}

	metrics.EXPECT().RecordHistogram(context.Background(), "app_mongo_stats", gomock.Any(), "hostname",
		gomock.Any(), "database", gomock.Any(), "type", gomock.Any())

	logger.EXPECT().Debug(gomock.Any())

	cl.logger = logger

	mt.Run("Drop", func(mt *mtest.T) {
		cl.Database = mt.DB
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		err := cl.Drop(context.Background(), mt.Coll.Name())

		assert.NoError(t, err)
	})
}

func TestClient_StartSession(t *testing.T) {
	// Create a connected client using the mock database
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metrics := NewMockMetrics(ctrl)
	logger := NewMockLogger(ctrl)

	cl := Client{metrics: metrics, tracer: otel.GetTracerProvider().Tracer("gofr-mongo")}

	// Set up the mock expectation for the metrics recording
	metrics.EXPECT().RecordHistogram(gomock.Any(), "app_mongo_stats", gomock.Any(), "hostname",
		gomock.Any(), "database", gomock.Any(), "type", gomock.Any()).Times(2)

	logger.EXPECT().Debug(gomock.Any()).Times(2)

	cl.logger = logger

	mt.Run("StartSessionCommitTransactionSuccess", func(mt *mtest.T) {
		cl.Database = mt.DB

		// Add mock responses if necessary
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		// Call the StartSession method
		sess, err := cl.StartSession()

		ses, ok := sess.(Transaction)
		if ok {
			err = ses.StartTransaction()
		}

		require.NoError(t, err)

		cl.Database = mt.DB
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		doc := map[string]any{"name": "Aryan"}

		resp, err := cl.InsertOne(context.Background(), mt.Coll.Name(), doc)

		assert.NotNil(t, resp)
		require.NoError(t, err)

		err = ses.CommitTransaction(context.Background())

		require.NoError(t, err)

		ses.EndSession(context.Background())

		// Assert that there was no error
		require.NoError(t, err)
	})
}

func Test_HealthCheck(t *testing.T) {
	// Create a connected client using the mock database
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metrics := NewMockMetrics(ctrl)
	logger := NewMockLogger(ctrl)

	cl := Client{metrics: metrics}

	cl.logger = logger

	mt.Run("HealthCheck Success", func(mt *mtest.T) {
		cl.Database = mt.DB
		mt.AddMockResponses(mtest.CreateSuccessResponse())

		resp, err := cl.HealthCheck(context.Background())

		require.NoError(t, err)
		assert.Contains(t, fmt.Sprint(resp), "UP")
	})

	mt.Run("HealthCheck Error", func(mt *mtest.T) {
		cl.Database = mt.DB
		mt.AddMockResponses(mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Index:   1,
			Code:    11000,
			Message: "duplicate key error",
		}))

		resp, err := cl.HealthCheck(context.Background())

		require.ErrorIs(t, err, errStatusDown)

		h, ok := resp.(*Health)
		require.True(t, ok)
		assert.Equal(t, statusDown, h.Status)
		assert.Contains(t, h.Details["error"], "duplicate key error")
	})
}

func notConnectedCalls() []struct {
	desc string
	call func(ctx context.Context, c *Client) error
} {
	return []struct {
		desc string
		call func(ctx context.Context, c *Client) error
	}{
		{"InsertOne", func(ctx context.Context, c *Client) error {
			_, err := c.InsertOne(ctx, "col", bson.M{"a": 1})
			return err
		}},
		{"InsertMany", func(ctx context.Context, c *Client) error {
			_, err := c.InsertMany(ctx, "col", []any{bson.M{"a": 1}})
			return err
		}},
		{"Find", func(ctx context.Context, c *Client) error {
			var res []bson.M
			return c.Find(ctx, "col", bson.M{}, &res)
		}},
		{"FindOne", func(ctx context.Context, c *Client) error {
			var res bson.M
			return c.FindOne(ctx, "col", bson.M{}, &res)
		}},
		{"UpdateByID", func(ctx context.Context, c *Client) error {
			_, err := c.UpdateByID(ctx, "col", 1, bson.M{"$set": bson.M{"a": 2}})
			return err
		}},
		{"UpdateOne", func(ctx context.Context, c *Client) error {
			return c.UpdateOne(ctx, "col", bson.M{}, bson.M{"$set": bson.M{"a": 2}})
		}},
		{"UpdateMany", func(ctx context.Context, c *Client) error {
			_, err := c.UpdateMany(ctx, "col", bson.M{}, bson.M{"$set": bson.M{"a": 2}})
			return err
		}},
		{"CountDocuments", func(ctx context.Context, c *Client) error {
			_, err := c.CountDocuments(ctx, "col", bson.M{})
			return err
		}},
		{"DeleteOne", func(ctx context.Context, c *Client) error {
			_, err := c.DeleteOne(ctx, "col", bson.M{})
			return err
		}},
		{"DeleteMany", func(ctx context.Context, c *Client) error {
			_, err := c.DeleteMany(ctx, "col", bson.M{})
			return err
		}},
		{"Drop", func(ctx context.Context, c *Client) error { return c.Drop(ctx, "col") }},
		{"CreateCollection", func(ctx context.Context, c *Client) error { return c.CreateCollection(ctx, "col") }},
	}
}

func TestClient_NotConnected(t *testing.T) {
	tests := append(notConnectedCalls(), struct {
		desc string
		call func(ctx context.Context, c *Client) error
	}{"StartSession", func(_ context.Context, c *Client) error {
		_, err := c.StartSession()
		return err
	}})

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			logger := NewMockLogger(ctrl)
			metrics := NewMockMetrics(ctrl)

			logger.EXPECT().Debug(gomock.Any()).AnyTimes()
			metrics.EXPECT().RecordHistogram(gomock.Any(), "app_mongo_stats", gomock.Any(),
				gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

			client := New(Config{})
			client.UseLogger(logger)
			client.UseMetrics(metrics)

			err := tc.call(t.Context(), client)

			require.ErrorIs(t, err, errNotConnected)
		})
	}
}

// TestClient_ServerUnreachable covers a valid config with no server listening: the client is kept, and every
// operation returns the driver's error (bounded by the caller's context) instead of panicking.
func TestClient_ServerUnreachable(t *testing.T) {
	ctrl := gomock.NewController(t)
	logger := NewMockLogger(ctrl)
	metrics := NewMockMetrics(ctrl)

	metrics.EXPECT().NewHistogram("app_mongo_stats", gomock.Any(), gomock.Any())
	metrics.EXPECT().RecordHistogram(gomock.Any(), "app_mongo_stats", gomock.Any(),
		"hostname", "127.0.0.1", "database", "test", "type", gomock.Any()).AnyTimes()
	logger.EXPECT().Debugf(gomock.Any(), gomock.Any(), gomock.Any())
	logger.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any())
	logger.EXPECT().Debug(gomock.Any()).AnyTimes()

	client := New(Config{URI: "mongodb://127.0.0.1:1/test", Database: "test", ConnectionTimeout: 100 * time.Millisecond})
	client.UseLogger(logger)
	client.UseMetrics(metrics)
	client.Connect()

	t.Cleanup(func() { _ = client.Client().Disconnect(context.Background()) })

	for _, tc := range notConnectedCalls() {
		t.Run(tc.desc, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			defer cancel()

			err := tc.call(ctx, client)

			require.Error(t, err)
			require.NotErrorIs(t, err, errNotConnected)
		})
	}

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	health, err := client.HealthCheck(ctx)

	require.ErrorIs(t, err, errStatusDown)

	h, ok := health.(*Health)
	require.True(t, ok)
	require.Equal(t, statusDown, h.Status)
	require.Equal(t, "127.0.0.1", h.Details["host"])
	require.Equal(t, "test", h.Details["database"])
	require.NotEmpty(t, h.Details["error"])
}

func TestClient_HealthCheck_NotConnected(t *testing.T) {
	client := New(Config{})

	health, err := client.HealthCheck(t.Context())

	require.ErrorIs(t, err, errNotConnected)
	require.Equal(t, &Health{Status: statusDown, Details: map[string]any{
		"host":     "",
		"database": "",
		"error":    errNotConnected.Error(),
	}}, health)
}
