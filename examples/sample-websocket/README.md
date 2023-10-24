# SAMPLE WEBSOCKET
A sample websocket app built using gofr.

## RUN
To run the app follow the below steps:

1. ` go run main.go`

This will start the server at port 9101.

## DOCKER BUILD
To Build a docker image, follow the below steps:

On non linux machines :
1. `GOOS=linux go build main.go` This will build a go binary
2. `docker build -t sample-websocket:$(date +%s) .`

On linux machines(Ubuntu/Mac):
1. `go build main.go` This will build a go binary
2. `docker build -t sample-websockt:$(date +%s) .`

## WebSocket Message Example

The request must contain the following header value pairs for the web-socket connection to be intialized:

| Header                | Value                         |
|-----------------------|-------------------------------|
| Connection            | upgrade                       |
| Upgrade               | websocket                     |
| Sec-Websocket-Version | 13                            |
| Sec-Websocket-Key     | 'base64 encoded random bytes' |

`Sec-Websocket-Key` is meant to prevent proxies from caching the request, by sending a random key. It does not provide any authentication.

####Follow the steps below to run the example:<br/>
```
1. Run the example using the command: go run main.go.
2. In browser, go to http://localhost:9101.
3. Send a message using the textbox and the send button which appears in the browser.
4. Messages sent from the browser appear on the terminal console.
```
