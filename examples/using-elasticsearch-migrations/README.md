# GoFr Elasticsearch Migration Example

This example demonstrates how to use GoFr's migration system to manage Elasticsearch indices and data migrations in an idempotent, production-ready way.

---

## Prerequisites
- **Go** 1.18+
- **Docker** (for running Elasticsearch)

---

## 1. Start Elasticsearch

Run Elasticsearch locally using Docker:
```sh
docker run -d --name elasticsearch -p 9200:9200 \
  -e "discovery.type=single-node" \
  -e "xpack.security.enabled=false" \
  elasticsearch:8.11.0
```

---

## 2. Run the Example App

```sh
cd examples/using-elasticsearch-migrations
go run main.go
```

---

## 3. Test Migration and Data Endpoints

### Check Migration Status
```sh
curl -s http://localhost:8000/migrations/status | jq
```
- You should see a list of migration records (with version numbers) if migrations ran successfully.

### Check Users Data
```sh
curl -s http://localhost:8000/users | jq
```
- You should see user documents (Alice, Bob, Carol) if the migrations and indexing worked.

### Health Check
```sh
curl -s http://localhost:8000/status
```
- Should return: `{ "status": "ok" }`

---

## 4. (Optional) Inspect Elasticsearch Directly

To verify the `users` index and data:
```sh
curl -s 'localhost:9200/users/_search?pretty'
```
To verify the `gofr_migrations` index:
```sh
curl -s 'localhost:9200/gofr_migrations/_search?pretty'
```

---

## How It Works
- **Migrations** are defined in Go and registered in `main.go`.
- **State** is tracked in the `gofr_migrations` index for idempotency.
- **All operations** use Elasticsearch's REST API via GoFr's datasource abstraction.
- **Endpoints** allow you to verify migration state and indexed data.

---

## Troubleshooting
- If you see `Internal Server Error`, ensure Elasticsearch is running and healthy (`curl http://localhost:9200/` should return JSON).
- If migrations do not appear, restart the app after confirming Elasticsearch is up.
- Check app logs for detailed error messages.

---

## Cleanup
To stop and remove the Elasticsearch container:
```sh
docker rm -f elasticsearch
``` 