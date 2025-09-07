package dynamodb

type Logger interface {
	Debug(args ...any)
	Debugf(pattern string, args ...any)
	Info(args ...any)
	Infof(pattern string, args ...any)
	Error(args ...any)
	Errorf(pattern string, args ...any)
}

type Log struct {
	Type     string `json:"type"`
	Duration int64  `json:"duration"`
	Key      string `json:"key"`
	Value    string `json:"value,omitempty"`
}

