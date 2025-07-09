FROM golang:1.21-alpine as builder
WORKDIR /app
COPY . .
RUN go build -o app main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/app .
EXPOSE 8080
CMD ["./app"] 