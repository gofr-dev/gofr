// This is auto-generated file using 'gofr migrate' tool. DO NOT EDIT.
package migrations

import (
	dbmigration "gofr.dev/cmd/gofr/migration/dbMigration"
)

func All() map[string]dbmigration.Migrator {
	return map[string]dbmigration.Migrator{

		"20220329122401": K20220329122401{},
		"20220329122459": K20220329122459{},
		"20220329122659": K20220329122659{},
		"20220329123813": K20220329123813{},
		"20220329123903": K20220329123903{},
		"20230518180017": K20230518180017{},
	}
}
