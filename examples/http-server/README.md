# HTTP Server Example

This GoFr example demonstrates a simple HTTP server which supports Redis and MySQL as datasources.

### To run the example follow the steps below:

- Run the docker image of the application
```console
docker-compose up -d
```

### To build and run the docker image

#### Build Docker image
- From the project root `/gofr`
```console
docker build -f examples/http-server/Dockerfile -t http-server:latest .
```
- Explanation:
    - `-f` `examples/http-server/Dockerfile` → path to the Dockerfile
    - `-t` `http-server:latest` → tag for the Docker image
    - `.` build context (project root; needed for go.mod and go.sum)

#### Run the Docker Container

```
docker run -p 9000:9000 --name http-server http-server:latest
```

- Explanation:
    - `-p 9000:9000` maps container port 9000 to host port 9000
    - `--name http-server` optional, gives your container a name


To test the example, follow these steps:

1. Open your browser and navigate to `http://localhost:9000/hello`.
2. To view the GoFr trace, open `https://tracer.gofr.dev` and paste the traceid.
3. To view the Grafana Dashboard open `http://localhost:3000`

