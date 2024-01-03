package log

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	
	"github.com/stretchr/testify/assert"
)

func TestEntryFromInputs(t *testing.T) {
	var hello struct{}

	testcases := []struct {
		desc          string
		format        string
		args          []interface{}
		expectedEntry *entry
	}{
		{"empty message", "", []interface{}{}, &entry{Data: map[string]interface{}{}, Message: ""}},
		{"args of length one", "", []interface{}{"hello logging"}, &entry{Data: map[string]interface{}{},
			Message: "hello logging"}},
		{"args of length two with empty format", "", []interface{}{"hello", "logging"}, &entry{Data: map[string]interface{}{},
			Message: "hello logging"}},
		{"args of length two with format", "hello %v %v", []interface{}{"logging", "yoyo"}, &entry{Data: map[string]interface{}{},
			Message: "hello logging yoyo"}},
		{"args of length two with strings", "", []interface{}{"hello", "%vlogging"}, &entry{Data: map[string]interface{}{},
			Message: "hello %vlogging"}},
		{"args of type struct and with format", "hello %v %v", []interface{}{hello, "logging"}, &entry{Data: map[string]interface{}{},
			Message: "hello {} logging"}},
		{"args of type struct and with empty format", "", []interface{}{hello, "logging"}, &entry{Data: map[string]interface{}{},
			Message: "logging"}},
	}

	for i, v := range testcases {
		e, _, _ := entryFromInputs(v.format, v.args...)

		assert.Equal(t, v.expectedEntry.Data, e.Data, "TEST[%d], Failed.\n%s", i, v.desc)

		assert.Equal(t, v.expectedEntry.Message, e.Message, "TEST[%d], Failed.\n%s", i, v.desc)
	}
}

func TestEntryFromInputErrorCase(t *testing.T) {
	var (
		channel = make(chan int)
		args    = []interface{}{&channel, "logging"}
		expMsg  = "logging"
	)

	formats := []string{"", "hello"}

	for i, v := range formats {
		e, _, _ := entryFromInputs(v, args...)

		if !strings.Contains(fmt.Sprintf("%v", e.Message), expMsg) {
			t.Errorf("TESTCASE [%d] Failed. Expected: %v\nGot: %v\n", i, expMsg, e.Message)
		}
	}
}

func TestEntryFromStringForJSON(t *testing.T) {
	tests := []struct {
		args       interface{}
		exp        entry
		expPerfLog bool
	}{
		{map[string]interface{}{"message": "hello", "correlationId": "test", "responseCode": 200},
			entry{
				Data:          map[string]interface{}{},
				CorrelationID: "test", Message: map[string]interface{}{"message": "hello", "correlationId": "test", "responseCode": 200},
			}, false,
		},
		{map[string]interface{}{"message": "hello", "correlationId": "test", "responseCode": 200, "method": "GET", "uri": "/temp", "duration": 5},
			entry{
				Data:          map[string]interface{}{"responseCode": 200.00, "method": "GET", "uri": "/temp", "duration": 5.00},
				CorrelationID: "test", Message: "hello",
			}, true,
		},
	}

	for i, tc := range tests {
		e, _, isPerfLog := entryFromInputs("", tc.args)

		assert.Equal(t, tc.exp.Data, e.Data, "TEST[%d], Failed.\n", i)

		assert.Equal(t, tc.exp.Message, e.Message, "TEST[%d], Failed.\n", i)

		assert.Equal(t, tc.exp.CorrelationID, e.CorrelationID, "TEST[%d], Failed.\n", i)

		assert.Equal(t, tc.expPerfLog, isPerfLog, "TEST[%d], Failed.\n", i)
	}
}

func TestEntry_TerminalOutput(t *testing.T) {
	now := time.Now()
	formattedNow := now.Format("15:04:05")
	appInfoData := &sync.Map{}
	appInfoData.Store("a", "b")

	appData := make(map[string]interface{})

	appData["a"] = "b"

	tests := []struct {
		input  entry
		output string
	}{
		// fatal error checking if msg and level is logged
		{entry{Level: Fatal, Message: "fatal error", Time: now},
			"FATA\u001B[0m [" + formattedNow + "]  fatal error"},
		// errorMessage
		{entry{Level: Fatal, Message: "fatal error", Time: now, Data: map[string]interface{}{"errorMessage": "error"}},
			fmt.Sprintf("\x1b[31mFATA\x1b[0m [%s]  error fatal error\n                \x1b[37m (Memory: <nil> GoRoutines: <nil>) \x1b[0m\n",
				formattedNow)},
		// DataQuery
		{entry{Level: Info, Message: "query field exists", Data: map[string]interface{}{"query": "query"}},
			"\x1b[36mINFO\x1b[0m [00:00:00]  query field exists query\n                \x1b[37m (Memory: <nil> GoRoutines: <nil>) \x1b[0m\n"},
		// correlationId
		{entry{Level: Info, CorrelationID: "test", Message: "hello"}, fmt.Sprintf(
			"INFO\u001B[0m [00:00:00]  hello\n%15s: %s", "CorrelationId", "test")},
		// data with message
		{entry{Level: Warn, Message: "hello", Data: map[string]interface{}{"name": "gofr.dev"}},
			"WARN\u001B[0m [00:00:00]  hello"},
		// statusCode
		{entry{Level: Warn, Message: "hello", Data: map[string]interface{}{"name": "gofr.dev", "responseCode": 200}},
			"WARN\u001B[0m [00:00:00]  hello"},
		// test data
		{entry{Level: Debug, Data: map[string]interface{}{"method": "get", "duration": 10000.0, "uri": "i", "datastore": "cql"}},
			fmt.Sprintf("\x1b[37mDEBU\x1b[0m [00:00:00] %s - %.2fms\n                \x1b[37m (Memory: <nil> GoRoutines: <nil>) \x1b[%vm\n",
				"cql", 10.0, 0)},
		// app data
		{entry{Level: Info, App: appInfo{Data: appData}, Message: "test"}, fmt.Sprintf(
			"INFO\u001B[0m [00:00:00]  test\n%15s: %v", "a", "b")},
	}

	for i, tc := range tests {
		output := tc.input.TerminalOutput()
		if !strings.Contains(output, tc.output) {
			t.Errorf("TESTCASE [%d] Failed. Got %v\tExpected %v\n", i, output, tc.output)
		}
	}
}

func Test_AppDataWithoutPersistence(t *testing.T) {
	b := new(bytes.Buffer)
	logger := NewMockLogger(b)

	tests := []struct {
		format          string
		values          []interface{}
		appData         map[string]interface{}
		expectedLog     string
		expectedAppData string
	}{
		{"Percent: %v %%", []interface{}{"5", map[string]interface{}{"key1": "value1"}}, nil,
			"Percent: 5 %", `"data":{"key1":"value1"}`},
		{"Hello %v", []interface{}{"gofr", map[string]interface{}{
			"key1": "value1"}, data{"Gofr", 1}}, map[string]interface{}{
			"key2": "value2"}, "Hello gofr", `"data":{"age":1,"key1":"value1","key2":"value2","name":"Gofr"}`},
		{"Hello %v", []interface{}{"gofr", map[string]interface{}{
			"key1": "value1"}, &data{"Rohan", 25}}, map[string]interface{}{
			"key2": "value2"}, "Hello gofr", `"data":{"age":25,"key1":"value1","key2":"value2","name":"Rohan"}`},
		{"Hello %v", []interface{}{"gofr", map[string]interface{}{"key1": "value1"}, nil}, map[string]interface{}{
			"key2": "value2"}, "Hello gofr", `"data":{"key1":"value1","key2":"value2"}`},
	}

	for i, v := range tests {
		b.Reset()

		for key, val := range v.appData {
			logger.AddData(key, val)
		}

		logger.Infof(v.format, v.values...)

		if !strings.Contains(b.String(), v.expectedLog) {
			t.Errorf("TESTACASE[%v] Failed, expected: %v, got: %v", i, v.expectedLog, b.String())
		}

		if !strings.Contains(b.String(), v.expectedAppData) {
			t.Errorf("TESTACASE[%v] Failed, expected: %v, got: %v", i, v.expectedAppData, b.String())
		}
	}
}
