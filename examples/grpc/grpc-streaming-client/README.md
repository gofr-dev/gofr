# gRPC Streaming Client Example

This GoFr example demonstrates a simple gRPC streaming server that communicates with another gRPC service hosted on a different machine. It serves as a client for another gRPC example included in this examples folder.
Refer to the documentation to setup

### Steps to Run the Example

1. First, start the corresponding `grpc-streaming-server` example, which is located at the relative path: `../grpc-streaming-server`.  
   Use the following command to start it:
   ```console
   go run main.go
   ```

2. Once the `grpc-streaming-server` is running, start this server using a similar command:
   ```console
   go run main.go
   ```