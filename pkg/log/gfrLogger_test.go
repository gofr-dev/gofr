package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogLevel(t *testing.T) {
	testcases := []struct {
		level  string
		output string
	}{

		{"warn", "DEBUG"}, // when log level is set to WARN, DEBUG log must not be logged
		{"fatal", "WARN"}, // when log level is set to FATAL, WARN or DEBUG log must not be logged
	}

	for i, v := range testcases {
		t.Setenv("LOG_LEVEL", v.level)

		b := new(bytes.Buffer)
		l := NewMockLogger(b)

		l.Warn("hello")
		l.Warnf("%d", 1)

		l.Debug("debug")
		l.Debugf("%s", v.level)

		if strings.Contains(v.output, b.String()) {
			t.Errorf("[TESTCASE%d]failed.expected %v\tgot %v\n", i+1, b.String(), v.output)
		}
	}
}

func TestLog(t *testing.T) {
	args := []interface{}{"hello"}

	tests := []struct {
		desc               string
		level              level
		correlationID      string
		correlationIDInMap string
		isTerminal         bool
		expLevel           string
	}{
		{"json encode output with log level: fatal", 1, "0", "", false, "FATAL"},
		{"json encode output with log level: error", 2, "1", "", false, "ERROR"},
		{"terminal output with log level: warn", 3, "2", "", true, "WARN"},
		{"terminal output with log level: info", 4, "3", "", true, "INFO"},
		{"terminal output with log level: info", 5, "4", "", true, "DEBU"},
		{"terminal output with log level: info", 5, "", "1", true, "DEBU"},
		{"invalid log level", 6, "5", "", false, ""},
	}

	for i, tc := range tests {
		b := new(bytes.Buffer)

		l := logger{correlationID: tc.correlationID, out: b, isTerminal: tc.isTerminal, rls: &levelService{level: 5}}
		syncData := sync.Map{}
		syncData.Store("correlationID", tc.correlationIDInMap)

		l.app = appInfo{
			syncData: &syncData,
		}
		l.log(tc.level, "%s", args)

		assert.Containsf(t, b.String(), tc.expLevel, "TESTCASE [%d] Failed. Expected %v\tGot %v\n", i, tc.expLevel, b.String())
	}
}

func TestLogger_Log_JSON(t *testing.T) {
	b := new(bytes.Buffer)
	l := NewMockLogger(b)

	r := struct {
		Name string
		Age  int
	}{
		"Alice", 23,
	}

	data, _ := json.Marshal(r)
	l.Log(string(data))

	expStr := `"message":"{\"Name\":\"Alice\",\"Age\":23}"`

	if !strings.Contains(b.String(), expStr) {
		t.Errorf("failed json log test .expected %v\tgot %v\n", expStr, b.String())
	}
}

func Test_CheckIfTerminal(t *testing.T) {
	_, err := os.Create("temp.txt")
	if err != nil {
		t.Errorf("File is not Cretead: %v", err)
	}

	file, _ := os.Open("temp.txt")

	testCases := []io.Writer{bytes.NewBuffer(nil), os.Stderr, file}
	defer os.Remove("temp.txt")

	for _, tc := range testCases {
		l := &logger{
			out: tc,
		}
		// Set terminal to ensure proper output format.
		l.isTerminal = checkIfTerminal(l.out)
		if l.isTerminal != false {
			t.Errorf("Output format is setted wrong on terminal")
		}
	}
}

type data struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func Test_DifferentLogFormats(t *testing.T) {
	b := new(bytes.Buffer)
	logger := NewMockLogger(b)

	testCases := []struct {
		format          string
		values          []interface{}
		appData         map[string]interface{}
		expectedLog     string
		expectedAppData string
	}{
		{"hello %v", []interface{}{"gofr"}, nil, "hello gofr", ""},
		{"hello %v%v", []interface{}{"gofr", "-v1"}, nil, "hello gofr-v1", ""},
		{"hello %v", []interface{}{"gofr", map[string]interface{}{
			"key1": "value1"}}, nil, "hello gofr", `"data":{"key1":"value1"}`},
		{"hello %v", []interface{}{"gofr", map[string]interface{}{
			"key1": "value1"}}, map[string]interface{}{
			"key2": "value2"}, "hello gofr", `"data":{"key1":"value1","key2":"value2"}`},
		{"hello %v", []interface{}{"gofr", map[string]interface{}{
			"key1": "value1"}, data{"Gofr", 1}}, map[string]interface{}{
			"key2": "value2"}, "hello gofr", `"data":{"age":1,"key1":"value1","key2":"value2","name":"Gofr"}`},
	}

	for i, v := range testCases {
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

func TestQueryLogs(t *testing.T) {
	b := new(bytes.Buffer)
	l := NewMockLogger(b)

	testCases := []struct {
		expectedLog []string
		duration    int64
	}{
		{[]string{"SELECT * FROM PERSONS"}, 5},
		{[]string{"SELECT * FROM PERSONS", "SELECT * FROM CUSTOMERS"}, 7},
	}

	for _, tc := range testCases {
		b.Reset()

		expected := fmt.Sprintf("%v", tc.expectedLog)
		q := struct {
			Query    string `json:"query"`
			Duration int64  `json:"duration"`
		}{expected, tc.duration}

		l.Debug(q)

		if !strings.Contains(b.String(), expected) {
			t.Errorf("FAILED, expected: %v, got: %v", expected, b.String())
		}

		expectedDurationLog := `"duration":` + strconv.Itoa(int(tc.duration))
		if !strings.Contains(b.String(), expectedDurationLog) {
			t.Errorf("FAILED, expected: %v, got: %v", expectedDurationLog, b.String())
		}
	}
}

func TestLevel(t *testing.T) {
	b := new(bytes.Buffer)
	l := NewMockLogger(b)

	l.Info("hello")

	if !strings.Contains(b.String(), `"level":"INFO"`) {
		t.Errorf("FAILED, ecpected level: INFO, got level: %v", b.String())
	}

	b.Reset()

	l.Error("hello")

	if !strings.Contains(b.String(), `"level":"ERROR"`) {
		t.Errorf("FAILED, ecpected level: ERROR, got level: %v", b.String())
	}

	b.Reset()

	l.Debug("hello")

	if !strings.Contains(b.String(), `"level":"DEBUG"`) {
		t.Errorf("FAILED, ecpected level: DEBUG, got level: %v", b.String())
	}

	b.Reset()

	l.Warn("hello")

	if !strings.Contains(b.String(), `"level":"WARN"`) {
		t.Errorf("FAILED, ecpected level: WARN, got level: %v", b.String())
	}

	b.Reset()

	l.Warn("hello", nil) // test for nil struct/map case

	if !strings.Contains(b.String(), `"level":"WARN"`) {
		t.Errorf("FAILED, ecpected level: WARN, got level: %v", b.String())
	}
}

// TestPerformanceLogMessage is testing logged message in case of performance log.
func TestPerformanceLogMessage(t *testing.T) {
	l := struct {
		Method   string `json:"method"`
		URI      string `json:"uri"`
		Duration int64  `json:"duration"`
	}{http.MethodGet, "/dummy", 1}

	b := new(bytes.Buffer)
	logger := NewMockLogger(b)

	logger.Info(l)

	if !strings.Contains(b.String(), "\"message\":\"GET /dummy\"") {
		t.Errorf("Failed to log the message in performance log")
	}
}

func Test_Errorf(t *testing.T) {
	b := new(bytes.Buffer)
	l := NewMockLogger(b)
	value := 1

	l.Errorf("Test error message: %v", value)

	assert.Contains(t, b.String(), "ERRO", "Test Failed:log level didn't match")
	assert.Contains(t, b.String(), "Test error message: 1", "Test Failed:error message didn't match")
}

func Test_Logf(t *testing.T) {
	b := new(bytes.Buffer)
	l := NewMockLogger(b)
	value := 1

	l.Logf("Test error message: %v", value)

	assert.Contains(t, b.String(), "INFO", "Test Failed:log level didn't match")
	assert.Contains(t, b.String(), "Test error message: 1", "Test Failed:error message didn't match")
}
