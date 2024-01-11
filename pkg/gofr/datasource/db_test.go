package datasource

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/testutil"
)

var (
	errDB = errors.New("DB error")
)

func getDB(t *testing.T) (*DB, sqlmock.Sqlmock) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	return &DB{mockDB, nil}, mock
}

func TestDB_SelectSingleColumnFromIntToString(t *testing.T) {
	db, mock := getDB(t)
	defer db.DB.Close()

	rows := sqlmock.NewRows([]string{"id"}).
		AddRow(1).
		AddRow(2)
	mock.ExpectQuery("^select id from users*").
		WillReturnRows(rows)

	ids := make([]string, 0)
	db.Select(context.TODO(), &ids, "select id from users")

	assert.Equal(t, []string{"1", "2"}, ids, "TEST Failed.\n")
}

func TestDB_SelectSingleColumnFromStringToString(t *testing.T) {
	db, mock := getDB(t)
	defer db.DB.Close()

	rows := sqlmock.NewRows([]string{"id"}).
		AddRow("1").
		AddRow("2")
	mock.ExpectQuery("^select id from users*").
		WillReturnRows(rows)

	ids := make([]string, 0)
	db.Select(context.TODO(), &ids, "select id from users")

	assert.Equal(t, []string{"1", "2"}, ids, "TEST Failed.\n")
}

func TestDB_SelectSingleColumnFromIntToInt(t *testing.T) {
	db, mock := getDB(t)
	defer db.DB.Close()

	rows := sqlmock.NewRows([]string{"id"}).
		AddRow(1).
		AddRow(2)
	mock.ExpectQuery("^select id from users*").
		WillReturnRows(rows)

	ids := make([]int, 0)
	db.Select(context.TODO(), &ids, "select id from users")

	assert.Equal(t, []int{1, 2}, ids, "TEST Failed.\n")
}

func TestDB_SelectSingleColumnFromIntToCustomInt(t *testing.T) {
	db, mock := getDB(t)
	defer db.DB.Close()

	rows := sqlmock.NewRows([]string{"id"}).
		AddRow(1).
		AddRow(2)
	mock.ExpectQuery("^select id from users*").
		WillReturnRows(rows)

	type CustomInt int

	ids := make([]CustomInt, 0)

	db.Select(context.TODO(), &ids, "select id from users")

	assert.Equal(t, []CustomInt{1, 2}, ids, "TEST Failed.\n")
}

func TestDB_SelectSingleColumnFromStringToCustomInt(t *testing.T) {
	db, mock := getDB(t)
	defer db.DB.Close()

	rows := sqlmock.NewRows([]string{"id"}).
		AddRow("1").
		AddRow("2")
	mock.ExpectQuery("^select id from users*").
		WillReturnRows(rows)

	type CustomInt int

	ids := make([]CustomInt, 0)

	db.Select(context.TODO(), &ids, "select id from users")

	assert.Equal(t, []CustomInt{1, 2}, ids, "TEST Failed.\n")
}

func TestDB_SelectContextError(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Microsecond))
	time.Sleep(1 * time.Millisecond)

	defer cancel()

	t.Setenv("LOG_LEVEL", "DEBUG")

	db, _ := getDB(t)
	defer db.DB.Close()

	// the query won't run, since context is past deadline and the function will simply return
	db.Select(ctx, nil, "select 1")
}

func TestDB_SelectDataPointerError(t *testing.T) {
	out := testutil.StderrOutputForFunc(func() {
		db, _ := getDB(t)
		defer db.DB.Close()

		db.Select(context.Background(), nil, "select 1")
	})

	assert.Contains(t, out, "We did not get a pointer. data is not settable.", "TEST Failed.\n")
}

func TestDB_SelectSingleColumnFromStringToCustomString(t *testing.T) {
	db, mock := getDB(t)
	defer db.DB.Close()

	rows := sqlmock.NewRows([]string{"id"}).
		AddRow("1").
		AddRow("2")
	mock.ExpectQuery("^select id from users*").
		WillReturnRows(rows)

	type CustomStr string

	ids := make([]CustomStr, 0)

	db.Select(context.TODO(), &ids, "select id from users")

	assert.Equal(t, []CustomStr{"1", "2"}, ids, "TEST Failed.\n")
}

func TestDB_SelectSingleRowMultiColumn(t *testing.T) {
	db, mock := getDB(t)
	defer db.DB.Close()

	rows := sqlmock.NewRows([]string{"id", "name", "image"}).
		AddRow("1", "Vikash", "http://via.placeholder.com/150")
	mock.ExpectQuery("^select 1 user*").
		WillReturnRows(rows)

	type user struct {
		Name  string
		ID    int
		Image string
	}

	u := user{}

	db.Select(context.TODO(), &u, "select 1 user")

	assert.Equal(t, user{
		Name:  "Vikash",
		ID:    1,
		Image: "http://via.placeholder.com/150",
	}, u, "TEST Failed.\n")
}

func TestDB_SelectSingleRowMultiColumnWithTags(t *testing.T) {
	db, mock := getDB(t)
	defer db.DB.Close()

	rows := sqlmock.NewRows([]string{"id", "name", "image_url"}).
		AddRow("1", "Vikash", "http://via.placeholder.com/150")
	mock.ExpectQuery("^select 1 user*").
		WillReturnRows(rows)

	type user struct {
		Name  string
		ID    int
		Image string `db:"image_url"`
	}

	u := user{}

	db.Select(context.TODO(), &u, "select 1 user")

	assert.Equal(t, user{
		Name:  "Vikash",
		ID:    1,
		Image: "http://via.placeholder.com/150",
	}, u, "TEST Failed.\n")
}

func TestDB_SelectMultiRowMultiColumnWithTags(t *testing.T) {
	db, mock := getDB(t)
	defer db.DB.Close()

	rows := sqlmock.NewRows([]string{"id", "name", "image_url"}).
		AddRow("1", "Vikash", "http://via.placeholder.com/150").
		AddRow("2", "Gofr", "")
	mock.ExpectQuery("^select users*").
		WillReturnRows(rows)

	type user struct {
		Name  string
		ID    int
		Image string `db:"image_url"`
	}

	users := []user{}

	db.Select(context.TODO(), &users, "select users")

	assert.Equal(t, []user{
		{
			Name:  "Vikash",
			ID:    1,
			Image: "http://via.placeholder.com/150",
		},
		{
			Name: "Gofr",
			ID:   2,
		},
	}, users, "TEST Failed.\n")
}

func TestDB_SelectSingleColumnError(t *testing.T) {
	ids := make([]string, 0)

	out := testutil.StderrOutputForFunc(func() {
		db, mock := getDB(t)
		defer db.DB.Close()

		mock.ExpectQuery("^select id from users").
			WillReturnError(errDB)

		db.Select(context.TODO(), &ids, "select id from users")
	})

	assert.Contains(t, out, "DB error", "TEST Failed.\n")

	assert.Equal(t, []string{}, ids, "TEST Failed.\n")
}

func TestDB_SelectDataPointerNotExpected(t *testing.T) {
	t.Setenv("LOG_LEVEL", "DEBUG")

	m := make(map[int]int)

	out := testutil.StdoutOutputForFunc(func() {
		db, _ := getDB(t)
		defer db.DB.Close()

		db.Select(context.Background(), &m, "select id from users")
	})

	assert.Contains(t, out, "a pointer to map was not expected.", "TEST Failed.\n")
}
