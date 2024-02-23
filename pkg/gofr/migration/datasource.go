package migration

type Datasource struct {
	Logger

	DB    db
	Redis commands
}
