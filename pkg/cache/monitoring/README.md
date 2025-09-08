# Pre-configured Monitoring for Gofr Cache

This directory contains a pre-configured Docker Compose setup to run Prometheus and Grafana for monitoring your application's cache metrics out-of-the-box.

## Features

- **Prometheus**: Pre-configured to scrape metrics from your Gofr application.
- **Grafana**: Pre-provisioned with a Prometheus data source and a dashboard for cache metrics.

## Quick Start

### 1. Prerequisites
- Docker and Docker Compose are installed.
- Your Gofr application is running and exposes Prometheus metrics on `/metrics` (this is default for Gofr apps).

### 2. Run the Monitoring Stack

Navigate to this directory and run:
```sh
docker-compose up --build
```
- **Grafana**: Will be available at [http://localhost:3000](http://localhost:3000) (user: `admin`, pass: `admin`)
- **Prometheus**: Will be available at [http://localhost:9090](http://localhost:9090)
- **App Metrics**: Your app should expose metrics at a port, e.g., [http://localhost:8080/metrics](http://localhost:8080/metrics)

### How it Works

The `prometheus.yml` is configured to scrape metrics from `host.docker.internal:8080`. `host.docker.internal` is a special DNS name that resolves to the host machine's IP address from within a Docker container. If your application runs on a different port, you can modify `prometheus.yml`.

The Grafana service is provisioned with the Prometheus data source and a dashboard defined in `provisioning/`.