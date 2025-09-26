# Build stage
FROM golang:1.25-alpine AS build
RUN apk add --no-cache build-base

WORKDIR /src

# Copy go.mod + go.sum first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Move into http-server folder and build the package (not main.go)
WORKDIR /src/examples/http-server
RUN CGO_ENABLED=0 go build -a -o /app/main .

# Final stage
FROM alpine:3.14
RUN apk add --no-cache tzdata ca-certificates
COPY --from=build /app/main /main
COPY --from=build /src/examples/http-server/configs /configs
EXPOSE 9000

CMD ["/main"]