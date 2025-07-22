package serrors

type ErrorSchema interface {
	Code() string
	SubCode() string
	Level() string
	Retryable() bool
	ExternalStatus() int
	ExternalMessage() string
	WithStatusCode(string) ErrorSchema
	WithSubCode(string) ErrorSchema
	WithLevel(Level) ErrorSchema
	WithRetryable(bool) ErrorSchema
	WithMeta(string, any) ErrorSchema
	WithMetaMulti(map[string]any) ErrorSchema
	WithExternalStatus(int) ErrorSchema
	WithExternalMessage(string) ErrorSchema
}
