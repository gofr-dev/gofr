# Pubsub
GoFr supports the following:
 - KAFKA(with Avro)
 - EventBridge
 - Google Pubsub 
 - Eventhub

To incorporate these, the respective configs must be set. Please refer to  [configs](/docs/new/configuration/introduction).

The client for any pubsub is established via the gofr context after gofr.New() is used to establish the connection by checking the configurations.

## Usage

**Publish Event**
```go
err := ctx.PublishEvent("", Person{
	ID:    id,
	Name:  "Rohan",
	Email: "rohan@email.xyz",
}, map[string]string{"test": "test"})
if err != nil {
	return nil, err
}
```
**Subscribe Event**
```go
p := Person{}

message, err := ctx.Subscribe(&p)
if err != nil {
	return nil, err
}
```
**Subscribe Event With Commit**
```go
// subscribe with commit
message, err := ctx.SubscribeWithCommit(func(message *pubsub.Message) (bool, bool) {
	// logic to decide if it has to be commited
})
```