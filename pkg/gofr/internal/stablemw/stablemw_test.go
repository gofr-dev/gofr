package stablemw

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func passThrough(next http.Handler) http.Handler { return next }

func TestMarkIsRecognisedAndNothingElse(t *testing.T) {
	marked := Mark(passThrough)

	assert.True(t, Is(marked))
	assert.True(t, Is(Mark(func(next http.Handler) http.Handler { return next })), "every Mark result carries the mark")

	assert.False(t, Is(nil))
	assert.False(t, Is(passThrough), "an unmarked constructor")
	assert.False(t, Is(func(next http.Handler) http.Handler { return marked(next) }), "a constructor that calls a marked one")
}

func TestMarkServesTheWrappedConstructor(t *testing.T) {
	var got http.Handler

	inner := http.NotFoundHandler()
	wrapped := http.NewServeMux()

	mw := Mark(func(next http.Handler) http.Handler {
		got = next

		return wrapped
	})

	assert.Same(t, wrapped, mw(inner))
	assert.NotNil(t, got, "the wrapped constructor was called")
}
