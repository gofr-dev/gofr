package serrors

type IError interface {
	Code() string
	SubCode() string
	Level() string
	Retryable() bool
	ExternalStatus() int
	ExternalMessage() string
	WithStatusCode(string) IError
	WithSubCode(string) IError
	WithLevel(Level) IError
	WithRetryable(bool) IError
	WithMeta(string, any) IError
	WithMetaMulti(map[string]any) IError
	WithExternalStatus(int) IError
	WithExternalMessage(string) IError
}
