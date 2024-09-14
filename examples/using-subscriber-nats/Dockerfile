# Build stage using Go 1.22
FROM golang:1.22 AS builder

# Create and set working directory
RUN mkdir /src/
WORKDIR /src/

# Copy all source files to the working directory
COPY . .

# Fetch Go module dependencies
RUN go get ./...

# Build the Go application statically
RUN go build -ldflags "-linkmode external -extldflags -static" -a main.go

# Final stage using Alpine
FROM alpine:latest

# Install required dependencies
RUN apk add --no-cache tzdata ca-certificates

# Copy the built binary from the builder stage
COPY --from=builder /src/main /main

# Copy the configs directory from the builder stage
COPY --from=builder /src/configs /configs

# Expose port 8200 for the GoFr service
EXPOSE 8200

# Command to run the main binary
CMD ["/main"]
