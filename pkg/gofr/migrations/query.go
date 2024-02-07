package migrations

const (
	createMySQLGoFrMigrationsTable = `CREATE TABLE gofr_migrations (
    app_name VARCHAR(50),
    version BIGINT,
    start_time TIMESTAMP,
    duration BIGINT,
    method VARCHAR(4)
);`

	checkMySQLGoFrMigrationsTable = `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'gofr_migrations');`

	getLastMySQLGoFrMigration = `SELECT COALESCE(MAX(version), 0) FROM gofr_migrations;`

	insertGoFrMigrationRow = `INSERT INTO gofr_migrations (app_name, version, start_time, method) VALUES (?, ?, ?, ?);`

	updateDurationInMigrationRecord = `UPDATE gofr_migrations SET duration = ? WHERE app_name = ? AND version = ? AND duration IS NULL AND method = 'UP';`
)
