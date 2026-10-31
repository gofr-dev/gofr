package middleware

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/websocket"
)

// The router builds a layer once per route only when its handler carries the marker. Losing it
// silently puts the allocation back on every request, so each framework middleware is pinned.
func TestFrameworkMiddlewaresAreStable(t *testing.T) {
	type stable interface{ GoFrStableMiddleware() }

	inner := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	routes := []string{}
	c := container.NewContainer(nil)

	for name, mw := range map[string]func(http.Handler) http.Handler{
		"Tracer":           Tracer,
		"Logging":          Logging(LogProbes{}, logging.NewMockLogger(logging.ERROR)),
		"CORS":             CORS(map[string]string{}, &routes),
		"Metrics":          Metrics(&mockMetrics{}),
		"WSHandlerUpgrade": WSHandlerUpgrade(c, websocket.New()),
	} {
		assert.Implements(t, (*stable)(nil), mw(inner), name)
	}
}
