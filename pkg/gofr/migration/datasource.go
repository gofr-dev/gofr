package migration

type Datasource struct {
	Logger

	DB sqlDB
}

func newDatasource(l Logger, db sqlDB) Datasource {
	return Datasource{
		Logger: l,
		DB:     db,
	}
}
