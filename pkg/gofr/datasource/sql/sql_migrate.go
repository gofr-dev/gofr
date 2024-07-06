package sql

import (
	"database/sql"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"gofr.dev/pkg/gofr/logging"
)

// GenerateCreateTableSQL generates a SQL CREATE TABLE statement for the given struct.
func GenerateCreateTableSQL(structType interface{}, dbType string, dropIfExists bool) (string, string, string, string, string, error) {
	dbType = strings.ToUpper(strings.TrimSpace(dbType))
	t := reflect.TypeOf(structType)
	tableName := ToSnakeCase(t.Name())

	fields := []string{}
	indexes := []string{}
	uniqueIndexes := []string{}
	foreignKeys := []string{}
	triggers := []string{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		columnName := ToSnakeCase(field.Name)

		sqlType := ""
		comment := ""
		checkConstraint := ""
		foreignKey := ""
		sqlTags := field.Tag.Get("sql")
		tagParts := strings.Split(sqlTags, ",")
		for _, tag := range tagParts {
			tag = strings.TrimSpace(tag)
			if strings.HasPrefix(tag, "comment(") {
				commentPattern := regexp.MustCompile(`comment\((.+)\)`)
				matches := commentPattern.FindStringSubmatch(tag)
				if len(matches) == 2 {
					comment = matches[1]
				}
			} else if strings.HasPrefix(tag, "check(") {
				checkPattern := regexp.MustCompile(`check\((.+)\)`)
				matches := checkPattern.FindStringSubmatch(tag)
				if len(matches) == 2 {
					checkConstraint = matches[1]
				}
			} else if strings.HasPrefix(tag, "fk(") {
				fkPattern := regexp.MustCompile(`fk\((.+):(.+)\)`)
				matches := fkPattern.FindStringSubmatch(tag)
				if len(matches) == 3 {
					foreignKey = fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s(%s)", ToSnakeCase(columnName), ToSnakeCase(matches[1]), ToSnakeCase(matches[2]))
				}
			}
		}

		switch field.Type.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			sqlType = "INTEGER"
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			sqlType = "INTEGER"
		case reflect.Float32, reflect.Float64:
			sqlType = "REAL"
		case reflect.Bool:
			sqlType = "BOOLEAN"
		case reflect.String:
			size := "255" // Default size
			for _, tag := range tagParts {
				if strings.HasPrefix(tag, "size(") {
					sizePattern := regexp.MustCompile(`size\((\d+)\)`)
					matches := sizePattern.FindStringSubmatch(tag)
					if len(matches) == 2 {
						size = matches[1]
					}
				}
			}
			sqlType = fmt.Sprintf("VARCHAR(%s)", size)
		case reflect.Struct:
			if field.Type == reflect.TypeOf(time.Time{}) {
				sqlType = "DATETIME"
			} else {
				sqlType = "TEXT"
			}
		default:
			sqlType = "TEXT" // Default to TEXT for any other types
		}

		for _, tag := range tagParts {
			switch tag {
			case "primary_key":
				sqlType += " PRIMARY KEY"
				if dbType == "MYSQL" && strings.Contains(sqlType, "INTEGER") {
					sqlType = strings.Replace(sqlType, "INTEGER", "INT", 1)
				}
			case "auto_increment":
				if dbType == "MYSQL" {
					sqlType += " AUTO_INCREMENT"
				} else if dbType == "POSTGRESQL" {
					sqlType += " SERIAL"
				}
			case "not_null":
				sqlType += " NOT NULL"
			case "unique":
				sqlType += " UNIQUE"
			case "index":
				indexes = append(indexes, fmt.Sprintf("CREATE INDEX idx_%s_%s ON %s (%s);\n\n\n", tableName, columnName, tableName, columnName))
			case "unique_index":
				uniqueIndexes = append(uniqueIndexes, fmt.Sprintf("CREATE UNIQUE INDEX uidx_%s_%s ON %s (%s);\n\n\n", tableName, columnName, tableName, columnName))
			}
		}

		fieldDef := fmt.Sprintf("%s %s", columnName, sqlType)
		if comment != "" {
			fieldDef += fmt.Sprintf(" COMMENT '%s'", comment)
		}
		if checkConstraint != "" {
			triggerName := fmt.Sprintf("check_%s_before_update", columnName)
			trigger := fmt.Sprintf("CREATE TRIGGER %s BEFORE UPDATE ON %s\nFOR EACH ROW\nBEGIN\n    IF %s THEN\n        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = '%s';\n    END IF;\nEND\n\n\n", triggerName, tableName, checkConstraint, checkConstraint)
			triggers = append(triggers, trigger)
		}

		fields = append(fields, fieldDef)

		if foreignKey != "" {
			foreignKeys = append(foreignKeys, foreignKey)
		}
	}

	dropTableStatement := ""
	if dropIfExists {
		dropTableStatement = fmt.Sprintf("DROP TABLE IF EXISTS %s;\n\n\n", tableName)
	}

	createTableStatement := fmt.Sprintf("CREATE TABLE %s (\n\t%s", tableName, strings.Join(fields, ",\n\t"))
	if len(foreignKeys) > 0 {
		createTableStatement += fmt.Sprintf(",\n\t%s", strings.Join(foreignKeys, ",\n\t"))
	}
	createTableStatement += "\n);\n\n\n"

	indexStatements := strings.Join(indexes, "\n")
	uniqueIndexStatements := strings.Join(uniqueIndexes, "\n")
	triggerStatements := strings.Join(triggers, "\n")

	//return fmt.Sprintf("%s\n%s\n%s\n%s\n%s", dropTableStatement, createTableStatement, indexStatements, uniqueIndexStatements, triggerStatements), nil
	return dropTableStatement, createTableStatement, indexStatements, uniqueIndexStatements, triggerStatements, nil
}

// ReverseStringArray reverses the content of a string array
func ReverseStringArray(input []string) []string {
	reversed := make([]string, len(input))
	for i, v := range input {
		reversed[len(input)-1-i] = v
	}
	return reversed
}

// executeSQLStatements executes a series of SQL statements provided as an array
func ExecuteMigrateJSONtoSQL(sqlDB *sql.DB, logger logging.Logger, sqlStatement string) error {
	sql := strings.TrimSpace(sqlStatement)
	if sql == "" {
		return nil
	}
	result, err := sqlDB.Exec(sql)
	if err != nil {
		logger.Errorf("error executing SQL: %v", err)
		return err
	}
	logger.Infof("result for %s: %v", sql, result)
	return nil
}
