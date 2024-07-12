package badger

import (
	"bytes"
	"context"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"os"
	"testing"
)

func setupDB(t *testing.T) *client {
	t.Helper()
	cl := New(Configs{DirPath: "test_badger"})

	var logs []byte

	ctrl := gomock.NewController(t)

	cl.UseLogger(NewMockLogger(DEBUG, bytes.NewBuffer(logs)))
	cl.UseMetrics(NewMockMetrics(ctrl))
	cl.Connect()

	t.Cleanup(func() {
		os.RemoveAll("test_badger")
	})

	return cl
}

func Test_ClientSet(t *testing.T) {
	cl := setupDB(t)
	defer os.RemoveAll("test_badger")

	err := cl.Set(context.Background(), "lkey", "lvalue")

	assert.NoError(t, err)
}

func Test_ClientGet(t *testing.T) {
	cl := setupDB(t)
	defer os.RemoveAll("test_badger")

	err := cl.Set(context.Background(), "lkey", "lvalue")

	val, err := cl.Get(context.Background(), "lkey")

	assert.NoError(t, err)
	assert.Equal(t, "lvalue", val)
}

func Test_ClientGetError(t *testing.T) {
	cl := setupDB(t)
	defer os.RemoveAll("test_badger")

	val, err := cl.Get(context.Background(), "lkey")

	assert.EqualError(t, err, "Key not found")
	assert.Empty(t, val)
}

func Test_ClientDeleteSuccessError(t *testing.T) {
	cl := setupDB(t)
	defer os.RemoveAll("test_badger")

	err := cl.Delete(context.Background(), "lkey")

	assert.NoError(t, err)
}
