package badger

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/stretchr/testify/assert"
)

func setupDB(t *testing.T) *client {
	t.Helper()
	cl := New(Configs{DirPath: t.TempDir()})

	var logs []byte

	ctrl := gomock.NewController(t)

	cl.UseLogger(NewMockLogger(DEBUG, bytes.NewBuffer(logs)))
	cl.UseMetrics(NewMockMetrics(ctrl))
	cl.Connect()

	return cl
}

func Test_ClientSet(t *testing.T) {
	cl := setupDB(t)

	err := cl.Set(context.Background(), "lkey", "lvalue")

	assert.NoError(t, err)
}

func Test_ClientGet(t *testing.T) {
	cl := setupDB(t)

	err := cl.Set(context.Background(), "lkey", "lvalue")

	val, err := cl.Get(context.Background(), "lkey")

	assert.NoError(t, err)
	assert.Equal(t, "lvalue", val)
}

func Test_ClientGetError(t *testing.T) {
	cl := setupDB(t)

	val, err := cl.Get(context.Background(), "lkey")

	assert.EqualError(t, err, "Key not found")
	assert.Empty(t, val)
}

func Test_ClientDeleteSuccessError(t *testing.T) {
	cl := setupDB(t)

	err := cl.Delete(context.Background(), "lkey")

	assert.NoError(t, err)
}

func Test_ClientHealthCheck(t *testing.T) {
	cl := setupDB(t)

	val, err := cl.HealthCheck(context.Background())

	assert.NoError(t, err)
	assert.Contains(t, fmt.Sprint(val), "UP")
}
