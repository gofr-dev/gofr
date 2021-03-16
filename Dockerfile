FROM golang:1.16

RUN mkdir -p /go/src/github.com/vikash/gofr
WORKDIR /go/src/github.com/vikash/gofr
COPY . .

RUN go build -ldflags "-linkmode external -extldflags -static" -a examples/simple-api/main.go

FROM alpine:latest
RUN apk add --no-cache tzdata ca-certificates
COPY --from=0 /go/src/github.com/vikash/gofr/main /main
EXPOSE 8000
CMD ["/main"]