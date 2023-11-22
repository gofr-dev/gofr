package log

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const defaultAppName = "gofr-app"

func TestNewLogger(t *testing.T) {
	l := newLogger()

	rls.init = false

	if l.app.Name != defaultAppName {
		t.Errorf("Expected APP_NAME: gofr-app     GOT:  %v", l.app.Name)
	}

	if l.app.Version != "dev" {
		t.Errorf("Expected APP_VERSION: dev    GOT:  %v", l.app.Version)
	}
}

func Test_NewLogger(t *testing.T) {
	l := &logger{
		out:        os.Stdout,
		app:        appInfo{},
		isTerminal: false,
	}

	l2 := NewLogger()

	val, ok := l2.(*logger)
	if !ok {
		t.Fatal("unable to typecast to type logger")
	}

	assert.Equal(t, l.out, val.out, "test failed as value didnot match")
	assert.IsType(t, l.app, val.app, "test failed as appInfo didnot match")
	assert.Equal(t, l.isTerminal, val.isTerminal, "test failed as isTerminal value didnot match")
}

func TestNewCorrelationLogger(t *testing.T) {
	l := &logger{
		out:           nil,
		app:           appInfo{},
		correlationID: "1234",
		isTerminal:    false,
	}

	log := NewCorrelationLogger("1234")

	val, ok := log.(*logger)
	if !ok {
		t.Fatal("unable to typecast to type logger")
	}

	assert.Equal(t, l.correlationID, val.correlationID, "test failed as log value didn't match")
}
