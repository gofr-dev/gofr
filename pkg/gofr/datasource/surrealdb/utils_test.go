package surrealdb

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func Test_clean(t *testing.T) {
	tests := []struct {
		desc  string
		input string
		want  string
	}{
		{desc: "empty string", input: "", want: ""},
		{desc: "collapses whitespace", input: "SELECT  *\n\tFROM   users", want: "SELECT * FROM users"},
		{desc: "trims surrounding whitespace", input: "  query  ", want: "query"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.want, clean(tc.input))
		})
	}
}

func TestQueryLog_PrettyPrint(t *testing.T) {
	tests := []struct {
		desc         string
		log          QueryLog
		expContains  []string
		expNamespace string
		expDatabase  string
	}{
		{
			desc:         "defaults applied for empty fields",
			log:          QueryLog{Query: "SELECT  *   FROM users", OperationName: "select", Duration: 12},
			expContains:  []string{"select", "SRLDB", "12", "default:default", "SELECT * FROM users"},
			expNamespace: defaultValue,
			expDatabase:  defaultValue,
		},
		{
			desc: "configured namespace and database are kept",
			log: QueryLog{Query: "INSERT INTO users", OperationName: "insert", Duration: 5,
				Namespace: "ns", Database: "db", ID: "1", Filter: "f", Update: "u"},
			expContains:  []string{"insert", "SRLDB", "db:ns", "INSERT INTO users"},
			expNamespace: "ns",
			expDatabase:  "db",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var buf bytes.Buffer

			tc.log.PrettyPrint(&buf)

			for _, s := range tc.expContains {
				assert.Contains(t, buf.String(), s)
			}

			assert.Equal(t, tc.expNamespace, tc.log.Namespace)
			assert.Equal(t, tc.expDatabase, tc.log.Database)
			assert.NotNil(t, tc.log.ID)
			assert.NotNil(t, tc.log.Filter)
			assert.NotNil(t, tc.log.Update)
		})
	}
}

func Test_isAdministrativeOperation(t *testing.T) {
	tests := []struct {
		desc  string
		query string
		want  bool
	}{
		{desc: "define statement", query: "DEFINE TABLE users;", want: true},
		{desc: "remove statement", query: "REMOVE TABLE users;", want: true},
		{desc: "namespace keyword", query: "INFO FOR NAMESPACE;", want: true},
		{desc: "database keyword", query: "INFO FOR DATABASE;", want: true},
		{desc: "data query", query: "SELECT * FROM users", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.want, isAdministrativeOperation(tc.query))
		})
	}
}

func Test_isCustomNil(t *testing.T) {
	tests := []struct {
		desc   string
		result any
		want   bool
	}{
		{desc: "custom nil value", result: models.CustomNil{}, want: true},
		{desc: "none value", result: models.None, want: true},
		{desc: "nil", result: nil, want: false},
		{desc: "string", result: "value", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.want, isCustomNil(tc.result))
		})
	}
}
