package migration

import (
	"sort"
	"strconv"

	db "gofr.dev/cmd/gofr/migration/dbMigration"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
)

// Migrate either runs UP or DOWN migration based in the `method` specified
func Migrate(app string, database db.DBDriver, migrations map[string]db.Migrator, method string, logger log.Logger) error {
	if database == nil {
		return &errors.Response{Reason: "no database specified"}
	}

	var (
		ranMigrations []string // used to keep ordered record of migrations run
		err           error
	)

	if method == "UP" {
		ranMigrations, err = runUP(app, database, migrations, logger)
		if err != nil {
			return err
		}
	} else {
		ranMigrations, err = runDOWN(app, database, migrations, logger)
		if err != nil {
			return err
		}
	}

	// inserts all the migrations ran to the database at once
	err = database.FinishMigration()
	if err != nil {
		return err
	}

	logger.Infof("Migration %v ran successfully: %v", method, ranMigrations)

	return nil
}

func runUP(app string, database db.DBDriver, migrations map[string]db.Migrator, logger log.Logger) ([]string, error) {
	var err error

	rm := make([]string, 0, len(migrations))

	// sort the migration based on timestamp, for version based migration, in ascending order
	keys := make([]string, 0, len(migrations))

	for k := range migrations {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	// fetch the max version ran, ensures version greater than the max version is only run
	lv := database.LastRunVersion(app, "UP")
	if lv == -1 {
		return nil, errors.DataStoreNotInitialized{DBName: getDBName(database)}
	}

	lvStr := strconv.Itoa(lv)

	for _, v := range keys {
		if v <= lvStr {
			continue
		}

		err = database.Run(migrations[v], app, v, "UP", logger)
		if err != nil {
			logger.Errorf("error occurred while running migration: %v, method: %v, error: %v", v, "UP", err)
			return nil, err
		}

		rm = append(rm, v)
	}

	return rm, nil
}

func runDOWN(app string, database db.DBDriver, migrations map[string]db.Migrator, logger log.Logger) ([]string, error) {
	var err error

	rm := make([]string, 0, len(migrations))
	keys := make([]string, 0, len(migrations))

	for k := range migrations {
		keys = append(keys, k)
	}

	// sort the migration based on the timestamp in descending order
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] > keys[j]
	})

	// fetch all UP and DOWN migrations already ran
	upMigrations, downMigrations := database.GetAllMigrations(app)
	if len(upMigrations) != 0 && upMigrations[0] == -1 {
		return nil, errors.DataStoreNotInitialized{DBName: getDBName(database)}
	}

	for _, v := range keys {
		// if migration DOWN is already run or migration UP of version `v` is not run, DOWN for version `v` will not run
		if contains(downMigrations, v) || !contains(upMigrations, v) {
			continue
		}

		err = database.Run(migrations[v], app, v, "DOWN", logger)
		if err != nil {
			logger.Errorf("error occurred while running migration: %v, method: %v, error: %v", v, "DOWN", err)
			return nil, err
		}

		rm = append(rm, v)
	}

	return rm, nil
}

func contains(slc []int, elem string) bool {
	for _, v := range slc {
		if elem == strconv.Itoa(v) {
			return true
		}
	}

	return false
}

func getDBName(database db.DBDriver) string {
	switch database.(type) {
	case *db.Mongo:
		return datastore.MongoStore
	case *db.Cassandra:
		return datastore.CassandraStore
	case *db.YCQL:
		return datastore.Ycql
	case *db.Redis:
		return datastore.RedisStore
	case *db.GORM:
		return datastore.SQLStore
	default:
		return "datastore"
	}
}
