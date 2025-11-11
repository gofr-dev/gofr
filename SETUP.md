# GoFr Local Setup Guide

This guide will help you set up the GoFr project on your local machine for development.

## Prerequisites

### Required Software
1. **Go 1.24 or higher** (Project uses Go 1.25)
   ```bash
   # Check your Go version
   go version
   
   # If you need to install/upgrade Go, visit: https://go.dev/dl/
   ```

2. **Docker** (for running dependent services)
   ```bash
   # Check Docker installation
   docker --version
   docker-compose --version
   ```

3. **Git**
   ```bash
   git --version
   ```

### Optional Tools
- **golangci-lint**: For code linting
  ```bash
  # macOS
  brew install golangci-lint
  
  # Or using Go
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
  ```

- **goimports**: For automatic import management
  ```bash
  go install golang.org/x/tools/cmd/goimports@latest
  ```

## Step 1: Clone the Repository

The project is already cloned at `/Users/opensource/Desktop/goll`. If you need to clone it elsewhere:

```bash
# Using HTTPS
git clone https://github.com/gofr-dev/gofr.git

# Using SSH
git clone git@github.com:gofr-dev/gofr.git

cd gofr
```

## Step 2: Install Go Dependencies

```bash
# From the project root directory
cd /Users/opensource/Desktop/goll

# Download all dependencies
go mod download

# Verify dependencies
go mod verify

# Tidy up (optional)
go mod tidy
```

## Step 3: Set Up Docker Services (Optional)

GoFr supports many databases and services. You only need to run the services you plan to use. Here are the most common ones:

### Essential Services

#### Redis (for caching)
```bash
docker run --name gofr-redis -p 6379:6379 -d redis:7.0.5
```

#### MySQL (for SQL database)
```bash
docker run --name gofr-mysql \
  -e MYSQL_ROOT_PASSWORD=password \
  -e MYSQL_DATABASE=test \
  -p 3306:3306 \
  -d mysql:8.0.30
```

#### PostgreSQL (alternative SQL database)
```bash
docker run --name gofr-pgsql \
  -e POSTGRES_DB=customers \
  -e POSTGRES_PASSWORD=root123 \
  -p 5432:5432 \
  -d postgres:15.1
```

#### Zipkin (for distributed tracing)
```bash
docker run --name gofr-zipkin \
  -d -p 9411:9411 \
  openzipkin/zipkin:2
```

### Additional Services (as needed)

#### MongoDB
```bash
docker run --name mongodb \
  -d -p 27017:27017 \
  -e MONGO_INITDB_ROOT_USERNAME=user \
  -e MONGO_INITDB_ROOT_PASSWORD=password \
  mongodb/mongodb-community-server:latest
```

#### Kafka
```bash
docker run --name kafka-1 -p 9092:9092 \
  -e KAFKA_ENABLE_KRAFT=yes \
  -e KAFKA_CFG_PROCESS_ROLES=broker,controller \
  -e KAFKA_CFG_CONTROLLER_LISTENER_NAMES=CONTROLLER \
  -e KAFKA_CFG_LISTENERS=PLAINTEXT://:9092,CONTROLLER://:9093 \
  -e KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT \
  -e KAFKA_CFG_ADVERTISED_LISTENERS=PLAINTEXT://127.0.0.1:9092 \
  -e KAFKA_CFG_AUTO_CREATE_TOPICS_ENABLE=true \
  -e KAFKA_BROKER_ID=1 \
  -e KAFKA_CFG_CONTROLLER_QUORUM_VOTERS=1@127.0.0.1:9093 \
  -e ALLOW_PLAINTEXT_LISTENER=yes \
  -e KAFKA_CFG_NODE_ID=1 \
  -v kafka_data:/bitnami \
  -d bitnami/kafka:3.4
```

#### Cassandra
```bash
docker run --name cassandra-node \
  -d -p 9042:9042 \
  -v cassandra_data:/var/lib/cassandra \
  cassandra:latest
```

#### NATS (for messaging)
```bash
docker run -d --name nats-server \
  -p 4222:4222 -p 8222:8222 \
  nats:latest -js
```

### Check Running Containers
```bash
docker ps
```

### Stop All Services
```bash
docker stop $(docker ps -q)
```

### Remove All Services
```bash
docker rm $(docker ps -aq)
```

## Step 4: Run Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...

# Run tests for a specific package
go test ./pkg/gofr/...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Step 5: Try Example Applications

### Basic HTTP Server
```bash
cd examples/http-server
go run main.go
```

Visit: http://localhost:8000

### HTTP Server with Redis
```bash
# Make sure Redis is running first
docker run --name gofr-redis -p 6379:6379 -d redis:7.0.5

cd examples/http-server-using-redis
go run main.go
```

### gRPC Example
```bash
cd examples/grpc
go run main.go
```

### WebSocket Example
```bash
cd examples/using-web-socket
go run main.go
```

### Cron Jobs Example
```bash
cd examples/using-cron-jobs
go run main.go
```

## Step 6: Build Your First GoFr Application

Create a new file `hello.go`:

```go
package main

import "gofr.dev/pkg/gofr"

func main() {
    app := gofr.New()

    app.GET("/hello", func(ctx *gofr.Context) (any, error) {
        return map[string]string{
            "message": "Hello from GoFr!",
            "status":  "success",
        }, nil
    })

    app.Run()
}
```

Run it:
```bash
go run hello.go
```

Test it:
```bash
curl http://localhost:8000/hello
```

## Step 7: Configure Your IDE

### VS Code
1. Install Go extension
2. Configure settings (`.vscode/settings.json`):
```json
{
    "go.lintTool": "golangci-lint",
    "go.lintOnSave": "package",
    "editor.formatOnSave": true,
    "[go]": {
        "editor.defaultFormatter": "golang.go"
    }
}
```

### GoLand / IntelliJ IDEA
1. Enable Go modules support
2. Configure file watchers for goimports
3. Enable golangci-lint integration

## Step 8: Environment Variables

GoFr uses environment variables for configuration. Create a `.env` file in your project:

```bash
# Application
APP_NAME=my-gofr-app
APP_VERSION=1.0.0
HTTP_PORT=8000

# Logging
LOG_LEVEL=INFO

# Tracing
TRACER_RATIO=1
TRACER_EXPORTER=zipkin
TRACER_URL=http://localhost:9411/api/v2/spans

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379

# MySQL
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=password
DB_NAME=test
DB_DIALECT=mysql

# PostgreSQL
# DB_HOST=localhost
# DB_PORT=5432
# DB_USER=postgres
# DB_PASSWORD=root123
# DB_NAME=customers
# DB_DIALECT=postgres
```

## Common Issues and Solutions

### Issue: Go version mismatch
```bash
# Solution: Update Go to 1.24+
brew upgrade go  # macOS
# or download from https://go.dev/dl/
```

### Issue: Port already in use
```bash
# Solution: Find and kill the process
lsof -ti:8000 | xargs kill -9

# Or change the port in your .env file
HTTP_PORT=8080
```

### Issue: Docker container conflicts
```bash
# Solution: Remove existing containers
docker rm -f gofr-redis gofr-mysql gofr-pgsql
```

### Issue: Module download fails
```bash
# Solution: Clear module cache and retry
go clean -modcache
go mod download
```

### Issue: Tests fail due to missing services
```bash
# Solution: Start required Docker services
# Check CONTRIBUTING.md for the complete list
```

## Useful Commands

### Development
```bash
# Run with auto-reload (using air)
go install github.com/cosmtrek/air@latest
air

# Format code
gofmt -w .
goimports -w .

# Lint code
golangci-lint run

# Check for typos
# Install: brew install typos-cli
typos

# Build binary
go build -o bin/myapp main.go
```

### Testing
```bash
# Run specific test
go test -run TestFunctionName

# Run tests with race detector
go test -race ./...

# Benchmark tests
go test -bench=. ./...

# Generate mocks (if using mockgen)
go generate ./...
```

### Docker Management
```bash
# View logs
docker logs gofr-redis
docker logs -f gofr-mysql  # Follow logs

# Execute commands in container
docker exec -it gofr-redis redis-cli
docker exec -it gofr-mysql mysql -uroot -ppassword

# Clean up everything
docker system prune -a
```

## Next Steps

1. **Read the Documentation**: Visit https://gofr.dev/docs
2. **Explore Examples**: Check the `examples/` directory
3. **Join the Community**: https://discord.gg/wsaSkQTdgq
4. **Contribute**: Read `CONTRIBUTING.md`
5. **Build Something**: Start creating your microservice!

## Quick Reference

### Project Structure
- `pkg/gofr/` - Core framework code
- `examples/` - Example applications
- `docs/` - Documentation source
- `go.mod` - Dependencies

### Important Files
- `README.md` - Project overview
- `CONTRIBUTING.md` - Contribution guidelines
- `project.md` - Detailed project documentation
- `SETUP.md` - This file

### Key URLs
- Documentation: https://gofr.dev/docs
- API Docs: https://pkg.go.dev/gofr.dev
- GitHub: https://github.com/gofr-dev/gofr
- Discord: https://discord.gg/wsaSkQTdgq

## Support

If you encounter issues:
1. Check the [GitHub Issues](https://github.com/gofr-dev/gofr/issues)
2. Ask on [Discord](https://discord.gg/wsaSkQTdgq)
3. Read the [Documentation](https://gofr.dev/docs)
4. Create a new issue with details

---

**Happy Coding with GoFr! 🚀**
