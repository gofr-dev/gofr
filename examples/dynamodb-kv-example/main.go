package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/kv-store/dynamodb"
)

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	// Check if DynamoDB Local is running
	if !isDynamoDBLocalRunning() {
		fmt.Println("❌ DynamoDB Local is not running!")
		fmt.Println("")
		fmt.Println("🔧 To run this example, you need to start DynamoDB Local first:")
		fmt.Println("   docker run --name dynamodb-local -d -p 8000:8000 amazon/dynamodb-local")
		fmt.Println("")
		fmt.Println("Then run this application again.")
		return
	}

	// Table should be created manually before running this application
	fmt.Println("📋 Using DynamoDB table: gofr-kv-store")

	// Now create the GoFr app
	app := gofr.New()

	// Create DynamoDB client with configuration for local development
	db := dynamodb.New(dynamodb.Configs{
		Table:            "gofr-kv-store",
		Region:           "us-east-1",
		Endpoint:         "http://localhost:8000", // For local DynamoDB
		PartitionKeyName: "pk",                    // Default is "pk" if not specified
	})

	// Inject the DynamoDB into gofr to use DynamoDB across the application
	// using gofr context
	app.AddKVStore(db)

	// Verify DynamoDB connection by testing health check
	fmt.Println("🔍 Verifying DynamoDB connection...")
	if _, err := db.HealthCheck(context.Background()); err != nil {
		log.Printf("Warning: DynamoDB health check failed: %v", err)
	} else {
		fmt.Println("✅ DynamoDB connection verified!")
	}

	// Register routes
	app.POST("/user", CreateUser)
	app.GET("/user/{id}", GetUser)
	app.PUT("/user/{id}", UpdateUser)
	app.DELETE("/user/{id}", DeleteUser)
	app.GET("/users", ListUsers)
	app.GET("/health", HealthCheck)

	// Add some sample data
	app.GET("/seed", SeedData)

	fmt.Println("🚀 GoFr DynamoDB Key-Value Store Example")
	fmt.Println("📊 Available endpoints:")
	fmt.Println("  POST   /user       - Create a new user")
	fmt.Println("  GET    /user/{id}  - Get user by ID")
	fmt.Println("  PUT    /user/{id}  - Update user by ID")
	fmt.Println("  DELETE /user/{id}  - Delete user by ID")
	fmt.Println("  GET    /users      - List all users")
	fmt.Println("  GET    /health     - Health check")
	fmt.Println("  GET    /seed       - Seed sample data")
	fmt.Println("")
	fmt.Println("🔗 DynamoDB Local running on http://localhost:8000")
	fmt.Println("🌐 API server starting on http://localhost:8001")
	fmt.Println("")

	app.Run()
}

func CreateUser(ctx *gofr.Context) (any, error) {
	var user User
	if err := ctx.Bind(&user); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}

	// Generate ID if not provided
	if user.ID == "" {
		user.ID = fmt.Sprintf("user_%d", time.Now().UnixNano())
	}
	user.CreatedAt = time.Now()

	// Serialize user to JSON string for storage using the helper function
	userData, err := dynamodb.ToJSON(user)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize user: %w", err)
	}

	// Store in DynamoDB using the standard KVStore interface
	if err := ctx.KVStore.Set(ctx, user.ID, userData); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	ctx.Logger.Infof("Created user with ID: %s", user.ID)
	return user, nil
}

func GetUser(ctx *gofr.Context) (any, error) {
	id := ctx.PathParam("id")
	if id == "" {
		return nil, fmt.Errorf("user ID is required")
	}

	// Get user from DynamoDB
	userData, err := ctx.KVStore.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Deserialize user from JSON string using the helper function
	var user User
	if err := dynamodb.FromJSON(userData, &user); err != nil {
		return nil, fmt.Errorf("failed to parse user data: %w", err)
	}

	ctx.Logger.Infof("Retrieved user: %s", user.Name)
	return user, nil
}

func UpdateUser(ctx *gofr.Context) (any, error) {
	id := ctx.PathParam("id")
	if id == "" {
		return nil, fmt.Errorf("user ID is required")
	}

	var user User
	if err := ctx.Bind(&user); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}

	user.ID = id

	// Serialize user to JSON string using the helper function
	userData, err := dynamodb.ToJSON(user)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize user: %w", err)
	}

	// Update in DynamoDB
	if err := ctx.KVStore.Set(ctx, id, userData); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	ctx.Logger.Infof("Updated user: %s", user.Name)
	return user, nil
}

func DeleteUser(ctx *gofr.Context) (any, error) {
	id := ctx.PathParam("id")
	if id == "" {
		return nil, fmt.Errorf("user ID is required")
	}

	// Delete from DynamoDB
	if err := ctx.KVStore.Delete(ctx, id); err != nil {
		return nil, fmt.Errorf("failed to delete user: %w", err)
	}

	ctx.Logger.Infof("Deleted user with ID: %s", id)
	return map[string]string{"message": "User deleted successfully", "id": id}, nil
}

func ListUsers(ctx *gofr.Context) (any, error) {
	// Note: This is a simplified implementation
	// In a real application, you might want to implement pagination
	// or use DynamoDB's scan operation with proper filtering

	ctx.Logger.Info("Listing users (simplified implementation)")
	return map[string]string{
		"message": "List users endpoint - in a real implementation, you would scan the DynamoDB table",
		"note":    "DynamoDB KVStore interface doesn't include a List operation. Consider using DynamoDB's scan or query operations directly for listing items.",
	}, nil
}

func HealthCheck(ctx *gofr.Context) (any, error) {
	// DynamoDB health check is automatically handled by GoFr
	ctx.Logger.Info("Health check requested")
	return map[string]string{
		"status":    "healthy",
		"service":   "dynamodb-kv-store",
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

func SeedData(ctx *gofr.Context) (any, error) {
	sampleUsers := []User{
		{
			ID:        "user_1",
			Name:      "John Doe",
			Email:     "john@example.com",
			CreatedAt: time.Now(),
		},
		{
			ID:        "user_2",
			Name:      "Jane Smith",
			Email:     "jane@example.com",
			CreatedAt: time.Now(),
		},
		{
			ID:        "user_3",
			Name:      "Bob Johnson",
			Email:     "bob@example.com",
			CreatedAt: time.Now(),
		},
	}

	createdCount := 0
	for _, user := range sampleUsers {
		userData, err := json.Marshal(user)
		if err != nil {
			ctx.Logger.Errorf("Failed to serialize user %s: %v", user.ID, err)
			continue
		}

		if err := ctx.KVStore.Set(ctx, user.ID, string(userData)); err != nil {
			ctx.Logger.Errorf("Failed to create user %s: %v", user.ID, err)
			continue
		}

		createdCount++
	}

	ctx.Logger.Infof("Seeded %d sample users", createdCount)
	return map[string]interface{}{
		"message": fmt.Sprintf("Successfully seeded %d sample users", createdCount),
		"users":   sampleUsers,
	}, nil
}

// Helper functions for DynamoDB Local management

func isDynamoDBLocalRunning() bool {
	// Check if container is running
	cmd := exec.Command("docker", "ps", "--filter", "name=dynamodb-local", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Check if the container is actually responding
	cmd = exec.Command("curl", "-s", "http://localhost:8000")
	err = cmd.Run()
	return err == nil && len(output) > 0
}


