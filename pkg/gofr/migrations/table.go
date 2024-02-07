package migrations

import "time"

type gofrMigration struct {
	AppName   string
	Version   int64
	StartTime time.Time
	Duration  time.Duration
	Method    string
}

const createGoFrMigrationsTable = `CREATE TABLE gofr_migrations (
    app_name VARCHAR(50),
    version BIGINT,
    start_time TIMESTAMP,
    duration BIGINT,
    method VARCHAR(4)
);`
