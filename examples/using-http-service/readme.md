# Http-Service Example

This GoFr example demonstrates an inter-service HTTP communication along with circuit-breaker as well as
service health config addition.

User can use the `AddHTTPService` method to add an HTTP service and then later get it using `GetHTTPService("service-name")`

### To run the example follow the below steps:
- Make sure your other service and health endpoint is ready and up on the given address.
- Now run the example using below command :

```console
go run main.go
```
