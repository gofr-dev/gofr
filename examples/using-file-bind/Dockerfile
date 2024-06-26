FROM golang:1.22

RUN mkdir /src/
WORKDIR /src/
COPY . .
RUN go get ./...
RUN go build -ldflags "-linkmode external -extldflags -static" -a main.go

FROM alpine:latest
RUN apk add --no-cache tzdata ca-certificates
COPY --from=0 /src/main /main
COPY --from=0 /src/configs /configs
EXPOSE 8300

CMD ["/main"]
