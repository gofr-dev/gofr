# Subscriber Example

This GoFr example demonstrates a simple Subscriber that subscribes asynchronously to given NATS streams and commits based on the handler response.

### To run the example, follow the steps below:

---

### **1. Run the NATS Server with JetStream Enabled**

Ensure you have Docker installed and run the NATS server using the following command:

```bash
docker run --name nats-server \
  -p 4222:4222 \
  -p 8222:8222 \
  -p 6222:6222 \
  -v /path/to/your/nats-server.conf:/etc/nats/nats-server.conf:ro \
  -v /path/to/your/local/data/jetstream:/data/jetstream \
  nats:latest -c /etc/nats/nats-server.conf
```
> Replace /path/to/your/nats-server.conf with the actual path to your NATS configuration file.
> Replace /path/to/your/local/data/jetstream with the actual path to where you want to store JetStream data on your local machine.

### **2. Build and Run the GoFr Application

Steps to build and run the Docker container:

#### **1. Build the Docker image:
    
    ```bash
    docker build -t gofr-subscriber-example ./using-subscriber-nats
    ```
#### **2. Run the Docker container:

    ```bash
    docker run --name gofr-subscriber -p 8200:8200 gofr-subscriber-example
    ```

#### **3. Create the Streams in NATS
Before running the subscriber, create the necessary NATS streams using the NATS CLI:

Create 'order-logs' Stream:
```bash
nats stream add order-logs --subjects "order-logs" --storage file
```

Create 'products' Stream:
```bash
nats stream add products --subjects "products" --storage file
```

#### **4. Run the Example

Once the streams are created, and the NATS server is running, you can run the GoFr application using the Docker 
container you've built.

```bash
docker run --name gofr-subscriber -p 8200:8200 gofr-subscriber-example
```

Your subscriber will now be listening to the order-logs and products streams.