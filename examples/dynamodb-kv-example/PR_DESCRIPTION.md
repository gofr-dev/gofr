# Add DynamoDB Key-Value Store Example

## Summary
This PR adds a comprehensive example demonstrating how to use DynamoDB as a key-value store with GoFr framework.

## Changes
- **New Example**: `examples/dynamodb-kv-example/main.go`
  - Complete CRUD operations (Create, Read, Update, Delete) for user management
  - DynamoDB Local integration with health checks
  - Sample data seeding endpoint
  - Proper error handling and logging
  - Docker-based DynamoDB Local setup validation

## Features
- ✅ **User Management**: Full CRUD operations for user entities
- ✅ **DynamoDB Integration**: Uses GoFr's KVStore interface with DynamoDB backend
- ✅ **Local Development**: Includes DynamoDB Local setup and validation
- ✅ **Health Checks**: Built-in health monitoring
- ✅ **Sample Data**: Seeding endpoint for testing
- ✅ **Error Handling**: Comprehensive error handling and logging

## API Endpoints
- `POST /user` - Create a new user
- `GET /user/{id}` - Get user by ID  
- `PUT /user/{id}` - Update user by ID
- `DELETE /user/{id}` - Delete user by ID
- `GET /users` - List all users (placeholder)
- `GET /health` - Health check
- `GET /seed` - Seed sample data

## Prerequisites
- Docker (for DynamoDB Local)
- Go 1.19+

## Setup
```bash
# Start DynamoDB Local
docker run --name dynamodb-local -d -p 8000:8000 amazon/dynamodb-local

# Run the example
go run main.go
```

## Testing
The example includes automatic DynamoDB Local detection and provides clear setup instructions if the service is not running.

