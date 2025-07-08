# Build stage
FROM golang:1.24 AS build

WORKDIR /go/src/app

# Copy the entire project
COPY . .

# Build the application
WORKDIR /go/src/app/examples/http-server
RUN go mod download
RUN go build -ldflags "-linkmode external -extldflags -static" -a -o /app/main main.go

# Final stage
FROM alpine:3.14
RUN apk add --no-cache tzdata ca-certificates
COPY --from=build /app/main /main
COPY --from=build /go/src/app/examples/http-server/configs /configs
EXPOSE 9000

CMD ["/main"]
