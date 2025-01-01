module gofr.dev/pkg/gofr/datasource/scylladb

go 1.23.3

replace github.com/gocql/gocql => github.com/scylladb/gocql v1.14.4

require (
	github.com/gocql/gocql v0.0.0-00010101000000-000000000000
	go.opentelemetry.io/otel/trace v1.33.0
)

require (
	github.com/golang/snappy v0.0.3 // indirect
	github.com/hailocab/go-hostpool v0.0.0-20160125115350-e80d13ce29ed // indirect
	go.opentelemetry.io/otel v1.33.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
)
