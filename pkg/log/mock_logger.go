package log

import (
	"io"
	"sync"
)

func NewMockLogger(output io.Writer) Logger {
	mu.Lock()

	rls.level = Debug

	mu.Unlock()

	return &logger{
		out: output,
		app: appInfo{
			Data:      make(map[string]interface{}),
			Framework: "gofr-" + GofrVersion,
			syncData:  &sync.Map{},
		},
	}
}
