package surrealdb

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ClientTestSuite defines the test suite.
type ClientTestSuite struct {
	suite.Suite
	client *Client
	ctx    context.Context
}

// Run the test suite.
func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientTestSuite))
}

// SetupSuite runs once before all tests.
func (s *ClientTestSuite) SetupSuite() {
	s.ctx = context.Background()
	config := Config{
		Host:       "localhost",
		Port:       8000,
		Username:   "root",
		Password:   "root",
		Namespace:  "test_namespace",
		Database:   "test_database",
		TLSEnabled: false,
	}
	s.client = New(config)
	s.client.logger = &mockLogger{}
	s.client.Connect()

	// Ensure the connection is established
	s.Require().NotNil(s.client.db, "Database connection should be established")
}

// TearDownSuite runs once after all tests.
func (s *ClientTestSuite) TearDownSuite() {
	if s.client != nil {
		err := s.client.Close()
		s.Require().NoError(err, "Closing the database connection should not error")
	}
}

// TearDownTest cleans up after each test.
func (s *ClientTestSuite) TearDownTest() {
	if s.client != nil && s.client.db != nil {
		_, _ = s.client.Query(s.ctx, "DELETE FROM users", nil)
	}
}

// TestConnection tests the connection functionality.
func (s *ClientTestSuite) TestConnection() {
	s.Run("successful connection", func() {
		client := New(Config{
			Host:      "localhost",
			Port:      8000,
			Username:  "root",
			Password:  "root",
			Namespace: "test_namespace",
			Database:  "test_database",
		})
		client.logger = &mockLogger{}
		client.Connect()
		s.Require().NotNil(client.db, "Database should be connected")
		err := client.Close()
		s.Require().NoError(err, "Closing the database connection should not error")
	})

	s.Run("invalid connection", func() {
		client := New(Config{
			Host: "invalid-host",
			Port: 8000,
		})
		client.logger = &mockLogger{}
		client.Connect()
		s.Require().Nil(client.db, "Database connection should be nil for an invalid host")
	})
}

// TestCreate tests the Create method.
func (s *ClientTestSuite) TestCreate() {
	s.Run("create record", func() {
		user := map[string]interface{}{
			"username": "testuser",
			"email":    "test@example.com",
		}

		result, err := s.client.Create(s.ctx, "users", user)
		s.Require().NoError(err, "Create should not return an error")
		s.Require().NotNil(result, "Create result should not be nil")
		s.Equal("testuser", result["username"], "Username should match")
		s.Equal("test@example.com", result["email"], "Email should match")
	})
}

// TestQuery tests the Query method.
func (s *ClientTestSuite) TestQuery() {
	s.Run("successful query", func() {
		user := map[string]interface{}{
			"username": "testuser",
			"email":    "test@example.com",
		}
		_, err := s.client.Create(s.ctx, "users", user)
		s.Require().NoError(err, "Create should not return an error")

		rawResult, err := s.client.Query(s.ctx, "SELECT * FROM users", nil)
		s.Require().NoError(err, "Query should not return an error")
		fmt.Println("Raw Query Result:", rawResult)

		results, err := s.client.Query(s.ctx, "SELECT * FROM users WHERE username = 'testuser'", nil)
		s.Require().NoError(err, "Query should not return an error")
		s.Require().NotEmpty(results, "Query results should not be empty")

		fmt.Println("Query Results:", results)

		firstResult, ok := results[0].(map[interface{}]interface{})
		s.Require().True(ok, "Result should be a map")

		s.Equal("testuser", firstResult["username"], "Username should match")
		s.Equal("test@example.com", firstResult["email"], "Email should match")
	})

	s.Run("empty result", func() {
		results, err := s.client.Query(s.ctx, "SELECT * FROM users WHERE username = 'nonexistent'", nil)
		s.Require().NoError(err, "Query should not return an error")
		s.Require().Empty(results, "Query results should be empty")
	})

	s.Run("query error", func() {
		results, err := s.client.Query(s.ctx, "INVALID QUERY", nil)
		s.Require().Error(err, "Query should return an error")
		s.Require().Nil(results, "Results should be nil in case of an error")
	})
}

// TestSelect tests the Select method.
func (s *ClientTestSuite) TestSelect() {
	s.Run("select records", func() {
		// Create a test record
		user := map[string]interface{}{
			"username": "testuser",
			"email":    "test@example.com",
		}
		_, err := s.client.Create(s.ctx, "users", user)
		s.Require().NoError(err, "Create should not return an error")

		results, err := s.client.Select(s.ctx, "users")
		s.Require().NoError(err, "Select should not return an error")
		s.Require().NotEmpty(results, "Select results should not be empty")
		s.Equal("testuser", results[0]["username"], "Username should match")
	})
}

// TestUpdate tests the Update method.
func (s *ClientTestSuite) TestUpdate() {
	s.Run("update record", func() {
		user := map[string]interface{}{
			"username": "testuser",
			"email":    "test@example.com",
		}
		result, err := s.client.Create(s.ctx, "users", user)
		s.Require().NoError(err, "Create should not return an error")
		s.Require().NotNil(result, "Created user should not be nil")

		updatedData := map[string]interface{}{
			"email": "updated@example.com",
		}
		record, _ := result["id"].(models.RecordID)

		id, ok := record.ID.(string)
		s.Require().True(ok, "ID should be a string")

		updated, err := s.client.Update(s.ctx, "users", id, updatedData)
		s.Require().NoError(err, "Update should not return an error")

		updatedUser := updated.(map[string]interface{})
		s.Equal("updated@example.com", updatedUser["email"], "Updated email should match")
	})
}

// TestDelete tests the Delete method.
func (s *ClientTestSuite) TestDelete() {
	s.Run("delete record", func() {
		user := map[string]interface{}{
			"username": "testuser",
			"email":    "test@example.com",
		}
		result, err := s.client.Create(s.ctx, "users", user)
		s.Require().NoError(err, "Create should not return an error")
		s.Require().NotNil(result, "Created user should not be nil")

		record, ok := result["id"].(models.RecordID)

		id, ok := record.ID.(string)
		s.Require().True(ok, "ID should be a string")

		_, err = s.client.Delete(s.ctx, "users", id)
		s.Require().NoError(err, "Delete should not return an error")

		results, err := s.client.Select(s.ctx, "users")
		s.Require().NoError(err, "Select should not return an error")
		s.Empty(results, "Select results should be empty after deletion")
	})
}

// Tests the HealthCheck method
func (s *ClientTestSuite) TestHealthCheck() {
	s.Run("health check success", func() {
		result, err := s.client.HealthCheck(s.ctx)

		s.Require().NoError(err, "HealthCheck should not return an error")

		health, ok := result.(*Health)

		s.Require().True(ok, "HealthCheck should return a Health struct")
		s.Equal("UP", health.Status, "Health status should be UP")
	})
}
