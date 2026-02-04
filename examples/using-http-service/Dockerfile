FROM golang:1.24

RUN mkdir /src/
WORKDIR /src/
COPY . .
RUN go get ./...
RUN go build -ldflags "-linkmode external -extldflags -static" -a main.go

FROM alpine:3.23.3
RUN apk add --no-cache tzdata ca-certificates
COPY --from=0 /src/main /main
COPY --from=0 /src/configs /configs
EXPOSE 9001

CMD ["/main"]
