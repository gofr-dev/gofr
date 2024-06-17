package sql

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"
)

// GenerateCreateTableSQL generates a SQL CREATE TABLE statement for the given struct.
func GenerateCreateTableSQL(structType interface{}, dbType string, dropIfExists bool) (string, error) {
	t := reflect.TypeOf(structType)
	tableName := ToSnakeCase(t.Name())

	fields := []string{}
	indexes := []string{}
	uniqueIndexes := []string{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		columnName := ToSnakeCase(field.Name)

		sqlType := ""
		comment := ""
		checkConstraint := ""
		gofrTags := field.Tag.Get("gofr")
		tagParts := strings.Split(gofrTags, ",")
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
				if dbType == "MySQL" && strings.Contains(sqlType, "INTEGER") {
					sqlType = strings.Replace(sqlType, "INTEGER", "INT", 1)
				}
			case "auto_increment":
				if dbType == "MySQL" {
					sqlType += " AUTO_INCREMENT"
				} else if dbType == "PostgreSQL" {
					sqlType += " SERIAL"
				}
			case "not_null":
				sqlType += " NOT NULL"
			case "unique":
				sqlType += " UNIQUE"
			case "index":
				indexes = append(indexes, fmt.Sprintf("CREATE INDEX idx_%s_%s ON %s (%s);", tableName, columnName, tableName, columnName))
			case "unique_index":
				uniqueIndexes = append(uniqueIndexes, fmt.Sprintf("CREATE UNIQUE INDEX uidx_%s_%s ON %s (%s);", tableName, columnName, tableName, columnName))
			}
		}

		fieldDef := fmt.Sprintf("%s %s", columnName, sqlType)
		if comment != "" {
			fieldDef += fmt.Sprintf(" COMMENT '%s'", comment)
		}
		if checkConstraint != "" {
			fieldDef += fmt.Sprintf(" CHECK (%s)", checkConstraint)
		}

		fields = append(fields, fieldDef)
	}

	dropTableStatement := ""
	if dropIfExists {
		dropTableStatement = fmt.Sprintf("DROP TABLE IF EXISTS %s;", tableName)
	}

	createTableStatement := fmt.Sprintf("CREATE TABLE %s (\n\t%s\n);", tableName, strings.Join(fields, ",\n\t"))
	indexStatements := strings.Join(indexes, "\n")
	uniqueIndexStatements := strings.Join(uniqueIndexes, "\n")

	return fmt.Sprintf("%s\n%s\n%s\n%s", dropTableStatement, createTableStatement, indexStatements, uniqueIndexStatements), nil
}
