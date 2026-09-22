package cassandra

import (
	"testing"

	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// closedCassandraSession returns a session wrapping a gocql session that is already closed,
// so every operation on it fails fast with gocql.ErrSessionClosed without needing a running cluster.
func closedCassandraSession() *cassandraSession {
	sess := &gocql.Session{}
	sess.Close()

	return &cassandraSession{session: sess}
}

func Test_CassandraSessionClosedOperations(t *testing.T) {
	tests := []struct {
		desc      string
		operation func(c *cassandraSession) error
		expErr    error
	}{
		{
			desc:      "query exec",
			operation: func(c *cassandraSession) error { return c.query("INSERT INTO users (id) VALUES (?)", 1).exec() },
			expErr:    gocql.ErrSessionClosed,
		},
		{
			desc: "query map scan cas",
			operation: func(c *cassandraSession) error {
				_, err := c.query("UPDATE users SET name = ? WHERE id = ? IF EXISTS", "a", 1).mapScanCAS(map[string]any{})

				return err
			},
			expErr: gocql.ErrSessionClosed,
		},
		{
			desc: "query scan cas",
			operation: func(c *cassandraSession) error {
				var name string

				_, err := c.query("UPDATE users SET name = ? WHERE id = ? IF EXISTS", "a", 1).scanCAS(&name)

				return err
			},
			expErr: gocql.ErrSessionClosed,
		},
		{
			desc: "execute batch",
			operation: func(c *cassandraSession) error {
				b := c.newBatch(gocql.LoggedBatch)
				b.Query("INSERT INTO users (id) VALUES (?)", 1)

				return c.executeBatch(b)
			},
			expErr: gocql.ErrSessionClosed,
		},
		{
			desc: "execute batch cas",
			operation: func(c *cassandraSession) error {
				b := c.newBatch(gocql.LoggedBatch)
				b.Query("INSERT INTO users (id) VALUES (?) IF NOT EXISTS", 1)

				_, err := c.executeBatchCAS(b)

				return err
			},
			expErr: gocql.ErrSessionClosed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.operation(closedCassandraSession())

			require.ErrorIs(t, err, tc.expErr)
		})
	}
}

func Test_CassandraIteratorOnClosedSession(t *testing.T) {
	tests := []struct {
		desc       string
		stmt       string
		expColumns []gocql.ColumnInfo
		expScan    bool
		expNumRows int
	}{
		{
			desc: "iterator of failed query has no rows",
			stmt: "SELECT id FROM users",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var id int

			iter := closedCassandraSession().query(tc.stmt).iter()

			assert.Equal(t, tc.expColumns, iter.columns())
			assert.Equal(t, tc.expScan, iter.scan(&id))
			assert.Equal(t, tc.expNumRows, iter.numRows())
		})
	}
}
