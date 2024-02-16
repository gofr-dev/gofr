package migration

type Datasource struct {
	Logger

	DB    sqlDB
	Redis redis
}

func newDatasource(l Logger, db sqlDB, r redis) Datasource {
	return Datasource{
		Logger: l,
		DB:     db,
		Redis:  r,
	}
}
