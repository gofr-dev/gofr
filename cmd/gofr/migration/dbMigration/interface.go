package dbmigration

import (
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/log"
)

const UP = "UP"

type DBDriver interface {
	/*
		Run creates the migration tracker table
		checks if dirty migrations exists for an `app`
		then executes either Up or Down migration based on the method parameter
	*/
	Run(mig Migrator, app, name, method string, logger log.Logger) error

	/*
		LastRunVersion fetches the last max version run
	*/
	LastRunVersion(app, method string) int

	/*
		GetAllMigrations fetches all the migrations run for an `app`
		separates out UP and DOWN migrations
	*/
	GetAllMigrations(app string) (upMigration []int, downMigration []int)

	/*
		FinishMigration stores all the migrations run into the database if, there is no error in any of the migrations
		error in one migration leads to not storing any of the migrations in the database
	*/
	FinishMigration() error
}

type Migrator interface {
	Up(db *datastore.DataStore, logger log.Logger) error
	Down(db *datastore.DataStore, logger log.Logger) error
}
