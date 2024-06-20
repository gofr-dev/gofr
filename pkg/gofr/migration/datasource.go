package migration

import "gofr.dev/pkg/gofr/datasource"

type Datasource struct {
	// TODO this should not be embedded rather it should be
	Logger

	SQL        SQL
	Redis      Redis
	PubSub     Client
	Clickhouse datasource.Clickhouse
}
