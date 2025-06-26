package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_InsertQuery_Success(t *testing.T) {
	tests := []struct {
		name        string
		dialect     string
		tableName   string
		fieldNames  []string
		values      []any
		constraints map[string]FieldConstraints
		expected    string
	}{
		{
			name:       "Basic INSERT (MySQL)",
			dialect:    "mysql",
			tableName:  "user",
			fieldNames: []string{"name", "age"},
			values:     []any{"John Doe", 30},
			expected:   "INSERT INTO `user` (`name`, `age`) VALUES (?, ?)",
		},
		{
			name:       "Basic INSERT (Postgres)",
			dialect:    "postgres",
			tableName:  "user",
			fieldNames: []string{"name", "age"},
			values:     []any{"John Doe", 30},
			expected:   `INSERT INTO "user" ("name", "age") VALUES ($1, $2)`,
		},
		{
			name:       "Skip Auto-Increment (MySQL)",
			dialect:    "mysql",
			tableName:  "user",
			fieldNames: []string{"id", "name"},
			values:     []any{1, "John Doe"},
			constraints: map[string]FieldConstraints{
				"id": {AutoIncrement: true},
			},
			expected: "INSERT INTO `user` (`name`) VALUES (?)",
		},
		{
			name:       "Skip Auto-Increment (Postgres)",
			dialect:    "postgres",
			tableName:  "user",
			fieldNames: []string{"id", "name"},
			values:     []any{1, "John Doe"},
			constraints: map[string]FieldConstraints{
				"id": {AutoIncrement: true},
			},
			expected: `INSERT INTO "user" ("name") VALUES ($2)`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := InsertQuery(tc.dialect, tc.tableName, tc.fieldNames, tc.values, tc.constraints)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_InsertQuery_Error(t *testing.T) {
	tests := []struct {
		name        string
		dialect     string
		tableName   string
		fieldNames  []string
		values      []any
		constraints map[string]FieldConstraints
	}{
		{
			name:       "NotNull Validation Error (MySQL)",
			dialect:    "mysql",
			tableName:  "user",
			fieldNames: []string{"name"},
			values:     []any{""},
			constraints: map[string]FieldConstraints{
				"name": {NotNull: true},
			},
		},
		{
			name:       "NotNull Validation Error (Postgres)",
			dialect:    "postgres",
			tableName:  "user",
			fieldNames: []string{"age"},
			values:     []any{0},
			constraints: map[string]FieldConstraints{
				"age": {NotNull: true},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := InsertQuery(tc.dialect, tc.tableName, tc.fieldNames, tc.values, tc.constraints)
			require.Error(t, err)
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

func Test_validateNotNull_Error(t *testing.T) {
	type customType struct{}

	tests := []struct {
		name        string
		fieldName   string
		value       any
		isNotNull   bool
		expectedErr string
	}{
		{
			name:        "Float null error",
			fieldName:   "weight",
			value:       0.0,
			isNotNull:   true,
			expectedErr: "field cannot be zero: weight",
		},
		{
			name:        "Nil channel",
			fieldName:   "channelField",
			value:       chan int(nil),
			isNotNull:   true,
			expectedErr: "field cannot be null: channelField",
		},
		{
			name:        "Custom type nil",
			fieldName:   "customField",
			value:       (*customType)(nil),
			isNotNull:   true,
			expectedErr: "field cannot be null: customField",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNotNull(tt.fieldName, tt.value, tt.isNotNull)
			require.EqualError(t, err, tt.expectedErr)
		})
	}
}
