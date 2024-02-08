package migrations

const (
	createMySQLGoFrMigrationsTable = `CREATE TABLE IF NOT EXISTS gofr_migrations (
    version BIGINT,
    method VARCHAR(4),
    start_time TIMESTAMP,
    duration BIGINT
);`

	checkMySQLGoFrMigrationsTable = `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'gofr_migrations');`

	getLastMySQLGoFrMigration = `SELECT COALESCE(MAX(version), 0) FROM gofr_migrations;`

	insertGoFrMigrationRow = `INSERT INTO gofr_migrations (version, method, start_time) VALUES (?, ?, ?);`

	updateDurationInMigrationRecord = `UPDATE gofr_migrations SET duration = ? WHERE version = ? AND method = 'UP' AND duration IS NULL;`
)
