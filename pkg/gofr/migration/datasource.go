package migration

type Datasource struct {
	Logger

	DB    sqlDB
	Redis redis
}
