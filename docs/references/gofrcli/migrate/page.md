---
description: "Generate timestamped database migration templates with an auto-maintained registry using the gofr migrate create command."
nextjs:
  metadata:
    title: "gofr migrate create — Generate Database Migration Templates"
    description: "Generate timestamped database migration templates with an auto-maintained registry using the gofr migrate create command."
---

# gofr migrate create

The migrate create command generates a migration template file with pre-defined structure in your migrations directory.
This boilerplate code helps you maintain consistent patterns when writing database schema modifications across your project.


## Command Usage
```bash
  gofr migrate create -name=<migration-name>
```

## Example Usage

```bash
gofr migrate create -name=create_employee_table
```
This command generates a migration directory which has the below files:

1. A new migration file with timestamp prefix (e.g., `20250127152047_create_employee_table.go`) containing:
```go
package migrations

import (
  "gofr.dev/pkg/gofr/migration"
)

func create_employee_table() migration.Migrate {
  return migration.Migrate{
    UP: func(d migration.Datasource) error {
      // write your migrations here
      return nil
    },
  }
}
```
2. An auto-generated all.go file that maintains a registry of all migrations:
```go
// This is auto-generated file using 'gofr migrate' tool. DO NOT EDIT.
package migrations

import (
  "gofr.dev/pkg/gofr/migration"
)

func All() map[int64]migration.Migrate {
  return map[int64]migration.Migrate {
    20250127152047: create_employee_table(),
  }
}
```

> **💡 Best Practice:** Learn about [organizing migrations by feature](/docs/advanced-guide/handling-data-migrations#organizing-migrations-by-feature) to avoid creating one migration per table or operation.

For detailed instructions on handling database migrations, see the [handling-data-migrations documentation](/docs/advanced-guide/handling-data-migrations)
For more examples, see the [using-migrations](https://github.com/gofr-dev/gofr/tree/main/examples/using-migrations)

---

## See also

- [GoFr CLI overview](/docs/references/gofrcli)
- [`gofr init`](/docs/references/gofrcli/init)
- [`gofr wrap grpc`](/docs/references/gofrcli/wrap-grpc)
- [`gofr store`](/docs/references/gofrcli/store)
