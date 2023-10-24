# SAMPLE HTTPS
A sample http app built using gofr.

## RUN
To run the app follow the below steps:

1. ` go run main.go`

This will start the server at port 9007.

## DOCKER BUILD
To Build a docker image, follow the below steps:

On non linux machines :
1. `GOOS=linux go build main.go` This will build a go binary
2. `docker build -t sample-https:$(date +%s) .`

On linux machines(Ubuntu/Mac):
1. `go build main.go` This will build a go binary
2. `docker build -t sample-https:$(date +%s) .`

## GENERATE TLS CERTIFICATE
Run the below command, from the project root directory, to generate the tls certificates

> cd configs; go run /usr/local/go/src/crypto/tls/generate_cert.go --rsa-bits 1024 --host 127.0.0.1,::1,localhost --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h; cd ..

