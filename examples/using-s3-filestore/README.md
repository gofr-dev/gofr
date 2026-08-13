# Using S3 File Store (MinIO)

This example demonstrates GoFr's file-store abstraction backed by an
S3-compatible object store. It registers an S3 file store with
`app.AddFileStore(...)` and exposes two HTTP endpoints:

- `POST /files?name=<key>` — writes the JSON body's `content` to the object `<key>`.
- `GET  /files?name=<key>` — reads the object `<key>` back.

The `<key>` may contain `/` separators (e.g. `uploads/2024/report.txt`); S3 has
no directories, so no parent prefix needs to exist beforehand.

## Running locally

Start a MinIO server (any S3-compatible backend works):

```bash
docker run -d --name minio \
  -e MINIO_ROOT_USER=minioadmin -e MINIO_ROOT_PASSWORD=minioadmin \
  minio/minio server /data
```

Point `configs/.env` (`S3_ENDPOINT`, `S3_BUCKET_NAME`, credentials) at it, make
sure the bucket exists, then:

```bash
go run .
```

```bash
curl -X POST 'http://localhost:8100/files?name=hello.txt' \
  -H 'Content-Type: application/json' -d '{"content":"hello gofr"}'

curl 'http://localhost:8100/files?name=hello.txt'
```

## Integration test

`main_test.go` runs the example end to end against a real MinIO backend. It is
the regression guard for
[gofr-dev/gofr#3804](https://github.com/gofr-dev/gofr/issues/3804): reading a
large object back over a real HTTP body, and creating the first object under a
fresh prefix — both are things the datasource's mock-based unit tests cannot
exercise. Set `S3_ENDPOINT` to reach your MinIO; the test skips itself if no
MinIO is reachable.
