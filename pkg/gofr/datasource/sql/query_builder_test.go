package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_InsertQuery(t *testing.T) {
	tableName := "user"
	fieldNames := []string{"id", "name"}

	tests := []struct {
		dialect  string
		expected string
	}{
		{
			dialect:  "mysql",
			expected: "INSERT INTO `user` (`id`, `name`) VALUES (?, ?)",
		},
		{
			dialect:  "postgres",
			expected: `INSERT INTO "user" ("id", "name") VALUES ($1, $2)`,
		},
	}

	for i, tc := range tests {
		t.Run(tc.dialect, func(t *testing.T) {
			actual := InsertQuery(tc.dialect, tableName, fieldNames)
			assert.Equal(t, tc.expected, actual, "TEST[%d], Failed.\n%s", i, tc.dialect)
		})
	}
}

func Test_SelectQuery(t *testing.T) {
	tableName := "user"
	tests := []struct {
		dialect  string
		expected string
	}{
		{
			dialect:  "mysql",
			expected: "SELECT * FROM `user`",
		},
		{
			dialect:  "postgres",
			expected: `SELECT * FROM "user"`,
		},
	}

	for i, tc := range tests {
		t.Run(tc.dialect, func(t *testing.T) {
			actual := SelectQuery(tc.dialect, tableName)
			assert.Equal(t, tc.expected, actual, "TEST[%d], Failed.\n%s", i, tc.dialect)
		})
	}
}

func Test_SelectByQuery(t *testing.T) {
	tableName := "user"
	field := "id"
	tests := []struct {
		dialect  string
		expected string
	}{
		{
			dialect:  "mysql",
			expected: "SELECT * FROM `user` WHERE `id`=?",
		},
		{
			dialect:  "postgres",
			expected: `SELECT * FROM "user" WHERE "id"=$1`,
		},
	}

	for i, tc := range tests {
		t.Run(tc.dialect, func(t *testing.T) {
			actual := SelectByQuery(tc.dialect, tableName, field)
			assert.Equal(t, tc.expected, actual, "TEST[%d], Failed.\n%s", i, tc.dialect)
		})
	}
}

func Test_UpdateByQuery(t *testing.T) {
	tableName := "user"
	fieldNames := []string{"name", "age"}
	field := "id"

	tests := []struct {
		dialect  string
		expected string
	}{
		{
			dialect:  "mysql",
			expected: "UPDATE `user` SET `name`=?, `age`=? WHERE `id`=?",
		},
		{
			dialect:  "postgres",
			expected: `UPDATE "user" SET "name"=$1, "age"=$2 WHERE "id"=$3`,
		},
	}

	for i, tc := range tests {
		t.Run(tc.dialect, func(t *testing.T) {
			actual := UpdateByQuery(tc.dialect, tableName, fieldNames, field)
			assert.Equal(t, tc.expected, actual, "TEST[%d], Failed.\n%s", i, tc.dialect)
		})
	}
}

func Test_DeleteByQuery(t *testing.T) {
	tableName := "user"
	field := "id"
	tests := []struct {
		dialect  string
		expected string
	}{
		{
			dialect:  "mysql",
			expected: "DELETE FROM `user` WHERE `id`=?",
		},
		{
			dialect:  "postgres",
			expected: `DELETE FROM "user" WHERE "id"=$1`,
		},
	}

	for i, tc := range tests {
		t.Run(tc.dialect, func(t *testing.T) {
			actual := DeleteByQuery(tc.dialect, tableName, field)
			assert.Equal(t, tc.expected, actual, "TEST[%d], Failed.\n%s", i, tc.dialect)
		})
	}
}
