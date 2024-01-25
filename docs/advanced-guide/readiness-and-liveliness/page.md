# Readiness and Liveliness
To check the liveliness and readiness of your service, use the following endpoints exposed by gofr application.

### Readiness 

Endpoint : `/.well-known/ready`

It will give the response if the service is either `UP` or `DOWN` with a `200` response code.

### Liveliness
Endpoint : `/.well-known/health`

It checks the stats of datasources and readiness of connected services.

It returns the following for these datasources : 
1. Redis - It contains the [stats](https://redis.io/commands/info/) field of redis INFO.

2. SQL - It contains the [stats](https://github.com/golang/go/blob/2c35def7efab9b8305487c23cb0575751642ce1e/src/database/sql/sql.go#L1183) for the sql.
