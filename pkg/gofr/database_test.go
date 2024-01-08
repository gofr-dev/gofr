package gofr

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"testing"
)

// a successful case
func Test_newMySqlPingFailure(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	db, err = newMYSQL(&dbConfig{HostName: "localhost", User: "root",
		Password: "password", Port: "3306", Database: "mysql"})

	assert.NotNil(t, err)
	assert.Nil(t, db)
}
