FROM golang:1.27

RUN mkdir -p /go/src/gofr.dev
WORKDIR /go/src/gofr.dev
COPY . .

RUN go build -ldflags "-linkmode external -extldflags -static" -a examples/http-server/main.go

FROM alpine:3.24
RUN apk add --no-cache tzdata ca-certificates
COPY --from=0 /go/src/gofr.dev/main /main
EXPOSE 8000
CMD ["/main"]
