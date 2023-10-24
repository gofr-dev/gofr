# tracing
A sample app to test tracing with grpc and http client.

## Zipkin Setup
Zipkin is used to see the traces. To run zipkin, follow the below steps:

1.   `docker run --name gofr-zipkin -d -p 2005:9411 openzipkin/zipkin:2`

2.  Open `http://localhost:2005/zipkin/` in browser.

3.  Enter the correlation-id of the request in the `search by trace ID` bar and click enter.

## Redis Setup
Run the following docker command to run redis
> `  docker run --name gofr-redis -p 2002:6379 -d redis:7.0.5`

## RUN
To run the app follow the below steps:

1. ` go run main.go`
2. Send request to `/trace` endpoint which will give correlation-id in response header.

- **_Note:_** `sample-api, sample-grpc` should be run before running `tracing`

This will start the server at port 9001.
