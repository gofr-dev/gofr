# AWS SNS
Amazon Simple Notification Service (Amazon SNS) is a fully managed messaging service for both application-to-application (A2A) and application-to-person (A2P) communication.

The A2A pub/sub functionality provides topics for high-throughput, push-based, many-to-many messaging between distributed systems, microservices, and event-driven serverless applications. 

Using Amazon SNS topics, your publisher systems can fanout messages to a large number of subscriber systems

* ### SUBSCRIBE
  SNS uses `SNS_TOPIC_ARN`, `SNS_PROTOCOL` and `SNS_ENDPOINT` in subcribing.
  SNS_PROTOCOL varies according to endpoint being subscribed like `email` for subscribing an email endpoint and `https\http` for subscribing to a server endpoint.

* ### PUBLISH
  SNS uses `SNS_TOPIC_ARN` to publish the data provided.


To run the example follow the steps below :

1) Run the below command from root directory to setup awssns locally for testing.

   > `bash ./examples/using-awssns/init.sh`

2) Now you can run the example on your PWD .

   > `go run main.go`
  