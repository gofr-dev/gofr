FROM alpine:latest

RUN mkdir -p /src/build
WORKDIR  /src/build

RUN apk add --no-cache tzdata ca-certificates

COPY ./main /main
COPY /configs /configs

RUN mkdir certificates
COPY /yugabytedb_zs_dev.crt certificates/
COPY /yugabytedb_zs_dev.key certificates/
COPY /yugabytedb_zs_dev_ca.crt certificates/

RUN chmod 600 certificates/yugabytedb_zs_dev.crt
RUN chmod 600 certificates/yugabytedb_zs_dev.key
RUN chmod 600 certificates/yugabytedb_zs_dev_ca.crt


EXPOSE 9095
CMD ["/main"]
