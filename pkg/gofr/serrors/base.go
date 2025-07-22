package serrors

import (
	"encoding/json"
)

func (err *Error) Code() string {
	return err.statusCode
}

func (err *Error) SubCode() string {
	return err.subStatusCode
}

func (err *Error) Level() string {
	return err.level.GetErrorLevel()
}

func (err *Error) Retryable() bool {
	return err.retryable
}

func (err *Error) ExternalStatus() int {
	return err.externalStatusCode
}

func (err *Error) ExternalMessage() string {
	return err.externalMessage
}

func (err *Error) WithStatusCode(value string) IError {
	err.statusCode = value
	return err
}

func (err *Error) WithSubCode(value string) IError {
	err.subStatusCode = value
	return err
}

func (err *Error) WithLevel(level Level) IError {
	err.level = level
	return err
}

func (err *Error) WithRetryable(retryable bool) IError {
	err.retryable = retryable
	return err
}

func (err *Error) WithMeta(key string, value any) IError {
	err.meta[key] = value
	return err
}

func (err *Error) WithMetaMulti(input map[string]any) IError {
	for key, value := range input {
		err.meta[key] = value
	}
	return err
}

func (err *Error) WithExternalStatus(code int) IError {
	err.externalStatusCode = code
	return err
}

func (err *Error) WithExternalMessage(msg string) IError {
	err.externalMessage = msg
	return err
}

func getMetaString(meta map[string]any) string {
	jsonBytes, err := json.Marshal(meta)
	if err != nil {
		panic("failed to marshal map to JSON: " + err.Error())
	}
	return string(jsonBytes)
}
