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

func Test_NewMongoClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metrics := NewMockMetrics(ctrl)
	logger := NewMockLogger(ctrl)

	logger.EXPECT().Debugf(gomock.Any(), gomock.Any())
	logger.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any())

	client := New(Config{Database: "test", Host: "localhost", Port: 27017, User: "admin", ConnectionTimeout: 1 * time.Second})
	client.Database = &mongo.Database{}
	client.UseLogger(logger)
	client.UseMetrics(metrics)
	client.Connect()

	assert.NotNil(t, client)
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

func Test_NewMongoClientError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metrics := NewMockMetrics(ctrl)
	logger := NewMockLogger(ctrl)

	logger.EXPECT().Errorf("error generating MongoDB URI: %v", gomock.Any())

	client := New(Config{Host: "mongo", Database: "test"})
	client.UseLogger(logger)
	client.UseMetrics(metrics)
	client.Connect()

	assert.Nil(t, client.Database)
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
		gomock.Any(), "database", gomock.Any(), "type", gomock.Any()).Times(3)

	logger.EXPECT().Debug(gomock.Any()).Times(3)

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
		gomock.Any(), "database", gomock.Any(), "type", gomock.Any())

	logger.EXPECT().Debug(gomock.Any())

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
		gomock.Any(), "database", gomock.Any(), "type", gomock.Any())

	logger.EXPECT().Debug(gomock.Any())

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

		err := cl.UpdateOne(context.Background(), mt.Coll.Name(), bson.D{{Key: "name", Value: "test"}}, bson.M{"$set": bson.M{"name": "testing"}})

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
		gomock.Any(), "database", gomock.Any(), "type", gomock.Any()).Times(2)

	logger.EXPECT().Debug(gomock.Any()).Times(2)

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

		assert.Contains(t, fmt.Sprint(resp), "DOWN")
	})
}
