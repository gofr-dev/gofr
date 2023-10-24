#### To run the example follow the steps below:

## MYSQL Setup

CASE 1:If you want to enable SSL,follow the below steps:

Step 1:Configure and Run the docker image of mysql on path `/zopsmart/gofr/examples/using-mysql` by

1. `sh init.sh`

Note: Include these configs in .env file:
```
DB_SSL=require
DB_CERTIFICATE_FILE=./certificateFile/client-cert.pem
DB_KEY_FILE=./certificateFile/client-key.pem
DB_CA_CERTIFICATE_FILE=./certificateFile/ca-cert.pem

```

CASE 2 :If we want to run example without SSL

Step 1:Run the docker image:

`  docker run --name gofr-mysql -e MYSQL_ROOT_PASSWORD=password -p 2001:3306 -d mysql:8.0.30`

Step 2:create database,run this on path `/zopsmart/gofr/examples/using-mysql`:

`docker exec -i gofr-ssl-mysql mysql -u root -ppassword < ../../.github/setups/setupSSL.sql`


## Run
Step 2:Now run the example on path `/zopsmart/gofr/examples/using-mysql` by

1. `go run main.go`


## Docker Build
To Build a docker image, follow the below steps:

On non linux machines :
1. `GOOS=linux go build main.go` This will build a go binary
2. `docker build -t using-mysql:$(date +%s) .`

On linux machines(Ubuntu/Mac):
1. `go build main.go` This will build a go binary
2. `docker build -t using-mysql:$(date +%s) .`

   