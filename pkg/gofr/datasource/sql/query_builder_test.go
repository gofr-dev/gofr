package sql

import (
	"testing"
	"time"
	"unsafe"

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
		{
			name:       "NotNull Validation Error (unsigned int)",
			dialect:    "mysql",
			tableName:  "user",
			fieldNames: []string{"views"},
			values:     []any{uint(0)},
			constraints: map[string]FieldConstraints{
				"views": {NotNull: true},
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

func Test_validateNotNull_UnsignedInt(t *testing.T) {
	// reflect.Value.Int() panics on unsigned kinds, so unsigned integers must be
	// validated via reflect.Value.Uint(). These calls previously panicked. Every
	// uint width listed in the type switch is exercised so none regress to the
	// default path.
	require.NoError(t, validateNotNull("views", uint(5), true))
	require.NoError(t, validateNotNull("views", uint8(5), true))
	require.NoError(t, validateNotNull("views", uint16(5), true))
	require.NoError(t, validateNotNull("views", uint32(5), true))
	require.NoError(t, validateNotNull("views", uint64(5), true))

	require.EqualError(t, validateNotNull("views", uint(0), true), "field cannot be zero: views")
	require.EqualError(t, validateNotNull("views", uint8(0), true), "field cannot be zero: views")
	require.EqualError(t, validateNotNull("views", uint16(0), true), "field cannot be zero: views")
	require.EqualError(t, validateNotNull("views", uint32(0), true), "field cannot be zero: views")
	require.EqualError(t, validateNotNull("views", uint64(0), true), "field cannot be zero: views")
}

func Test_validateNotNull_NonNillableKinds(t *testing.T) {
	// reflect.Value.IsNil() panics on non-nillable kinds, so the default path
	// must guard against them. These kinds can never be null and previously
	// panicked; they are now accepted for a NOT NULL field.
	require.NoError(t, validateNotNull("active", true, true))
	require.NoError(t, validateNotNull("active", false, true))
	require.NoError(t, validateNotNull("createdAt", time.Time{}, true))
	require.NoError(t, validateNotNull("offset", uintptr(0), true))

	// A genuinely nil value for a NOT NULL field is still reported as null.
	require.EqualError(t, validateNotNull("data", []byte(nil), true), "field cannot be null: data")
	require.EqualError(t, validateNotNull("meta", map[string]int(nil), true), "field cannot be null: meta")
	require.EqualError(t, validateNotNull("value", nil, true), "field cannot be null: value")

	// reflect.Value.IsNil is documented for six kinds but accepts UnsafePointer as well, so a nil
	// one is reported as null instead of reaching the default path and being accepted.
	var nilUnsafe unsafe.Pointer

	require.EqualError(t, validateNotNull("ptr", nilUnsafe, true), "field cannot be null: ptr")
	require.NoError(t, validateNotNull("ptr", unsafe.Pointer(&nilUnsafe), true))
}

// namedCount and namedName stand in for the named types an entity struct normally uses. A type
// switch matches only the predeclared types, so before the dispatch moved to reflect.Kind these
// missed every case and fell through to the default path. On development that path called IsNil
// on them and panicked; once the default path was guarded they stopped panicking but were
// silently ACCEPTED, so a named zero passed a NOT NULL column that its underlying kind rejects.
// Dispatching on the kind is what makes Count and uint answer the same.
type (
	namedCount uint
	namedAge   int
	namedName  string
)

func Test_validateNotNull_NamedTypes(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr string
	}{
		{"named uint, zero", namedCount(0), "field cannot be zero: f"},
		{"named uint, set", namedCount(7), ""},
		{"named int, zero", namedAge(0), "field cannot be zero: f"},
		{"named int, set", namedAge(30), ""},
		{"named string, empty", namedName(""), "field cannot be empty: f"},
		{"named string, set", namedName("gofr"), ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateNotNull("f", tc.value, true)

			if tc.wantErr == "" {
				require.NoError(t, err, "a named type must validate like its underlying kind")

				return
			}

			require.EqualError(t, err, tc.wantErr, "a named type must validate like its underlying kind")
		})
	}
}

// Test_validateNotNull_Floats closes the non-zero float branch, which no other test reached.
func Test_validateNotNull_Floats(t *testing.T) {
	require.NoError(t, validateNotNull("price", float32(1.5), true))
	require.NoError(t, validateNotNull("price", 1.5, true))
	require.EqualError(t, validateNotNull("price", float32(0), true), "field cannot be zero: price")
	require.EqualError(t, validateNotNull("price", 0.0, true), "field cannot be zero: price")

	// isNotNull false short-circuits before any reflection, whatever the value.
	require.NoError(t, validateNotNull("price", nil, false))
}

// Test_validateNotNull_SignedWidthsAndNegatives closes three gaps the other tests leave open.
//
// Only plain int was exercised, so dropping any of int8/16/32 from the dispatch went unnoticed --
// they would fall to validateDefaultNotNull and a zero would be accepted for a NOT NULL column.
//
// And no negative value was tested anywhere, so the zero check could be relaxed from `== 0` to
// `<= 0` with the suite still green. That mutation is not hypothetical-looking: `<= 0` reads like
// a reasonable "must be positive" rule, and it would silently reject every negative value in a
// column that is perfectly entitled to hold one.
func Test_validateNotNull_SignedWidthsAndNegatives(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr string
	}{
		{"int8 zero", int8(0), "field cannot be zero: f"},
		{"int16 zero", int16(0), "field cannot be zero: f"},
		{"int32 zero", int32(0), "field cannot be zero: f"},
		{"int64 zero", int64(0), "field cannot be zero: f"},
		{"int8 negative", int8(-5), ""},
		{"int16 negative", int16(-5), ""},
		{"int32 negative", int32(-5), ""},
		{"int64 negative", int64(-5), ""},
		{"int negative", -5, ""},
		{"float negative", -0.5, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateNotNull("f", tc.value, true)

			if tc.wantErr == "" {
				require.NoError(t, err, "a negative value is not a zero value and must be accepted")

				return
			}

			require.EqualError(t, err, tc.wantErr, "every signed width must reach validateIntNotNull")
		})
	}
}

// Test_validateNotNull_NilFunc covers the reflect.Func arm of isNillableKind, which no other test
// reaches. A nil func in a NOT NULL field is a null like any other nillable kind.
func Test_validateNotNull_NilFunc(t *testing.T) {
	var fn func()

	require.EqualError(t, validateNotNull("cb", fn, true), "field cannot be null: cb")
	require.NoError(t, validateNotNull("cb", func() {}, true))
}
