FROM alpine:latest

RUN mkdir -p /src/build
WORKDIR  /src/build

RUN apk add --no-cache tzdata ca-certificates

COPY ./main /main
COPY /configs /configs

EXPOSE 1443
CMD ["/main"]