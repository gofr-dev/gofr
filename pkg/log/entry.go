package log

import (
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/gookit/color"
)

type entry struct {
	Level         level                  `json:"level"`
	Time          time.Time              `json:"time"`
	Message       interface{}            `json:"message"`
	System        map[string]interface{} `json:"system"`
	App           appInfo                `json:"app"`
	CorrelationID string                 `json:"correlationId,omitempty"`
	Data          map[string]interface{} `json:"data,omitempty"` // Used to support middleware log data
}

// TerminalOutput returns a string representation of the log entry for terminal output.
func (e *entry) TerminalOutput() string {
	levelColor := e.Level.colorCode()

	if runtime.GOOS == "windows" {
		color.New()
	}

	s := fmt.Sprintf("\u001B[%dm%s\u001B[0m [%s] ", levelColor, e.Level.String()[0:4], e.Time.Format("15:04:05"))

	s = populateData(e, s)

	for k, v := range e.App.Data {
		s += fmt.Sprintf("\n%15s: %v", k, v)
	}

	// Add some system stats
	if e.CorrelationID != "" {
		s += fmt.Sprintf("\n%15s: %s \u001B[%dm(Memory: %v GoRoutines: %v) \u001B[0m",
			"CorrelationId", e.CorrelationID, normalColor, e.System["alloc"], e.System["goRoutines"])
	} else {
		s += fmt.Sprintf("\n%15s \u001B[%dm (Memory: %v GoRoutines: %v) \u001B[0m", "", normalColor, e.System["alloc"], e.System["goRoutines"])
	}

	return s + "\n"
}

func populateDatastoreAndMessage(e *entry) string {
	s := ""
	if e.Message != nil { // http client sends message on error
		s += fmt.Sprintf(" %s", e.Message)
	}

	if db, ok := e.Data["datastore"]; ok {
		s = fmt.Sprintf("%v", db)
	}

	if v, ok := e.Data["query"]; ok {
		s += fmt.Sprintf(" %v", v)
	}

	return s
}

// entryFromInputs takes multiple strings and creates a log entry from it.
// It is written separately so different use-cases can be tested safely in isolation.
// format represents the formatting of the values passed in args similar to fmt.Sprintf()
//
//nolint:gocognit,gocyclo // cannot reduce the complexity further
func entryFromInputs(format string, args ...interface{}) (e *entry, data map[string]interface{}, isPerformanceLog bool) {
	e = &entry{
		Time: time.Now(),
		Data: make(map[string]interface{}),
	}

	// No need for array if size is only 1
	if len(args) == 1 {
		j, hashMap := isJSON(args[0])
		if j {
			_, methodOK := hashMap["method"]
			_, uriOK := hashMap["uri"]
			_, durationOK := hashMap["duration"]

			if methodOK && uriOK && durationOK {
				e.Data = hashMap
				isPerformanceLog = true
				e.Message = fmt.Sprint(hashMap["method"], " ", hashMap["uri"])
			} else if durationOK { // condition for query logging, duration and query should be present under Data
				e.Data = hashMap
			} else {
				if format != "" {
					e.Message = fmt.Sprintf(format, args[0])
				} else {
					e.Message = args[0]
				}
			}

			if cID, ok := hashMap["correlationId"]; ok {
				e.CorrelationID = cID.(string)

				delete(hashMap, "correlationId")
			}

			if m, ok := hashMap["message"]; ok && isPerformanceLog {
				e.Message = m.(string)

				delete(hashMap, "message")
			}
		} else {
			if format != "" {
				e.Message = fmt.Sprintf(format, args[0])
			} else {
				e.Message = args[0]
			}
		}
	} else {
		var (
			msg    string
			values []interface{}
		)

		// Count the number of format specifiers in the log.
		//nolint:gomnd // not a magic number
		countFormat := strings.Count(format, "%") - 2*strings.Count(format, "%%")

		for i, v := range args {
			if i < countFormat && format != "" {
				values = append(values, v)
			} else if v != nil { // fixes the panic, if `v` is nil
				//nolint:exhaustive // need to handle only struct, map and ptr separately
				switch reflect.TypeOf(v).Kind() {
				case reflect.Struct, reflect.Map:
					dataBytes, _ := json.Marshal(v)
					_ = json.Unmarshal(dataBytes, &data)
				case reflect.Ptr:
					dataBytes, _ := json.Marshal(v)
					err := json.Unmarshal(dataBytes, &data)
					if err != nil { // To handle the different types of pointer objects.
						if format != "" {
							values = append(values, v)
						} else {
							msg += fmt.Sprintf(" %v", v)
						}
					}
				default:
					if format != "" {
						values = append(values, v)
					} else {
						msg += fmt.Sprintf(" %v", v)
					}
				}
			}
		}

		if format != "" {
			msg = fmt.Sprintf(format, values...)
		}

		msg = strings.TrimPrefix(msg, " ")
		e.Message = msg
	}

	return e, data, isPerformanceLog
}

func populateData(e *entry, s string) string {
	if len(e.Data) > 0 {
		if v, ok := e.Data["errorMessage"]; ok {
			s += fmt.Sprintf(" %v", v)
		}

		s += populateDatastoreAndMessage(e)

		duration, ok := e.Data["duration"]
		if ok {
			durationf64, _ := duration.(float64)
			s += fmt.Sprintf(" - %.2fms", durationf64/1000) //nolint:gomnd,gomnd // 1000 is used to convert the microsecond to milliseconds
		}

		statusCode, ok := e.Data["responseCode"]
		if ok {
			s += fmt.Sprintf(" (StatusCode: %v)", statusCode)
		}
	} else {
		s += fmt.Sprintf(" %v", e.Message)
	}

	return s
}
