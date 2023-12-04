package log

import (
	"io"
	"sync"
)

func NewMockLogger(output io.Writer) Logger {
	return &logger{
		rls: levelService{level: Debug},
		out: output,
		app: appInfo{
			Data:      make(map[string]interface{}),
			Framework: "gofr-" + GofrVersion,
			syncData:  &sync.Map{},
		},
	}
}
