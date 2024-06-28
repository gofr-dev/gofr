FROM golang:1.22

RUN mkdir -p /go/src/gofr.dev
WORKDIR /go/src/gofr.dev
COPY . .

RUN go build -ldflags "-linkmode external -extldflags -static" -a examples/http-server/main.go

FROM alpine:latest
RUN apk add --no-cache tzdata ca-certificates
COPY --from=0 /go/src/gofr.dev/main /main
EXPOSE 8000
CMD ["/main"]
