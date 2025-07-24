package serrors

import (
	"encoding/json"
	"fmt"
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

func (err *Error) WithStatusCode(value string) ErrorSchema {
	err.statusCode = value
	return err
}

func (err *Error) WithSubCode(value string) ErrorSchema {
	err.subStatusCode = value
	return err
}

func (err *Error) WithLevel(level Level) ErrorSchema {
	err.level = level
	return err
}

func (err *Error) WithRetryable(retryable bool) ErrorSchema {
	err.retryable = retryable
	return err
}

func (err *Error) WithMeta(key string, value any) ErrorSchema {
	err.meta[key] = value
	return err
}

func (err *Error) WithMetaMulti(input map[string]any) ErrorSchema {
	for key, value := range input {
		err.meta[key] = value
	}

	return err
}

func (err *Error) WithExternalStatus(code int) ErrorSchema {
	err.externalStatusCode = code
	return err
}

func (err *Error) WithExternalMessage(msg string) ErrorSchema {
	err.externalMessage = msg
	return err
}

func getMetaString(meta map[string]any) string {
	jsonBytes, err := json.Marshal(meta)

	if err != nil {
		fmt.Println("failed to marshal map to JSON", err.Error())
	}

	return string(jsonBytes)
}
