# Build stage
FROM golang:1.24 AS build

WORKDIR /src

COPY . .
RUN go get ./...
# Build the application
RUN go build -ldflags "-linkmode external -extldflags -static" -a -o /app/main main.go

# Final stage
FROM alpine:3.14
RUN apk add --no-cache tzdata ca-certificates
COPY --from=build /app/main /main
COPY --from=build /src/configs /configs
EXPOSE 9000

CMD ["/main"]
