// This is auto-generated file using 'gofr migrate' tool. DO NOT EDIT.
package migrations

import (
	"gofr.dev/cmd/gofr/migration/dbMigration"
)

func All() map[string]dbmigration.Migrator {
	return map[string]dbmigration.Migrator{

		"20231218131933": K20231218131933{},
	}
}
