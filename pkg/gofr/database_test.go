package gofr

import (
	"database/sql"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_newMySQL(t *testing.T) {
	testCases := []struct {
		desc   string
		port   string
		expDB  *sql.DB
		expErr error
	}{
		{"db connected successfully", "3306", &sql.DB{}, nil},
		{"db connection  failed", "2001", nil, &net.OpError{}},
	}

	for i, tc := range testCases {
		db, err := newMYSQL(&dbConfig{HostName: "localhost", User: "root",
			Password: "password", Port: tc.port, Database: "mysql"})

		assert.IsType(t, tc.expDB, db, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.IsType(t, tc.expErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
