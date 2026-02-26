# HTTP Server Example

This GoFr example demonstrates a simple HTTP server which supports Redis and MySQL as datasources.

### To run the example, follow the steps below:

#### 1. Run with Docker Compose (recommended)

From the project root (`/gofr`):

```console
docker compose -f examples/http-server/docker/docker-compose.yml up -d
```

* Explanation:

    * `-f examples/http-server/docker/docker-compose.yml` → path to the docker-compose file
    * `up -d` → builds (if needed) and runs services in detached mode

---

#### 2. Build & Run Manually (without docker-compose)

##### Build the Docker image

From the project root (`/gofr`):

```console
docker build -f examples/http-server/Dockerfile -t http-server:latest .
```

* Explanation:

    * `-f examples/http-server/Dockerfile` → path to the Dockerfile
    * `-t http-server:latest` → tag for the Docker image
    * `.` → build context (project root; needed for `go.mod` and `go.sum`)

##### Run the Docker container

```console
docker run -p 9000:9000 --name http-server http-server:latest
```

* Explanation:
    * `-p 9000:9000` → maps container port 9000 to host port 9000
    * `--name http-server` → optional, gives your container a name

* Use **Compose** when you want the whole stack (app + Redis + MySQL + Grafana + Prometheus).
* Use **Docker build/run** when you just want to run the app container alone.

To test the example, follow these steps:

1. Open your browser and navigate to `http://localhost:9000/hello`.
2. To view the GoFr trace, open `https://tracer.gofr.dev` and paste the traceid.
3. To access the Grafana Dashboard, open `http://localhost:3000`. The dashboard UI will be displayed. Use the default admin credentials to log in:
    - Username: `admin`
    - Password: `password`
