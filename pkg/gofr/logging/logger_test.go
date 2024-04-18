package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/term"

	"gofr.dev/pkg/gofr/testutil"
)

func TestLogger_LevelInfo(t *testing.T) {
	printLog := func() {
		logger := NewLogger(INFO)
		logger.Debug("Test Debug Log")
		logger.Info("Test Info Log")
		logger.Error("Test Error Log")
	}

	infoLog := testutil.StdoutOutputForFunc(printLog)
	errLog := testutil.StderrOutputForFunc(printLog)

	assertMessageInJSONLog(t, infoLog, "Test Info Log")
	assertMessageInJSONLog(t, errLog, "Test Error Log")

	if strings.Contains(infoLog, "DEBUG") {
		t.Errorf("TestLogger_LevelInfo Failed. DEBUG log not expected ")
	}
}

func TestLogger_LevelError(t *testing.T) {
	printLog := func() {
		logger := NewLogger(ERROR)
		logger.Logf("%s", "Test Log")
		logger.Debugf("%s", "Test Debug Log")
		logger.Infof("%s", "Test Info Log")
		logger.Errorf("%s", "Test Error Log")
	}

	infoLog := testutil.StdoutOutputForFunc(printLog)
	errLog := testutil.StderrOutputForFunc(printLog)

	assert.Equal(t, "", infoLog) // Since log level is ERROR we will not get any INFO logs.
	assertMessageInJSONLog(t, errLog, "Test Error Log")
}

func TestLogger_LevelDebug(t *testing.T) {
	printLog := func() {
		logger := NewLogger(DEBUG)
		logger.Logf("Test Log")
		logger.Debug("Test Debug Log")
		logger.Info("Test Info Log")
		logger.Error("Test Error Log")
	}

	infoLog := testutil.StdoutOutputForFunc(printLog)
	errLog := testutil.StderrOutputForFunc(printLog)

	if !(strings.Contains(infoLog, "DEBUG") && strings.Contains(infoLog, "INFO")) {
		// Debug Log Level will contain all types of logs i.e. DEBUG, INFO and ERROR
		t.Errorf("TestLogger_LevelDebug Failed!")
	}

	assertMessageInJSONLog(t, errLog, "Test Error Log")
}

func TestLogger_LevelNotice(t *testing.T) {
	printLog := func() {
		logger := NewLogger(NOTICE)
		logger.Log("Test Log")
		logger.Debug("Test Debug Log")
		logger.Info("Test Info Log")
		logger.Notice("Test Notice Log")
		logger.Error("Test Error Log")
	}

	infoLog := testutil.StdoutOutputForFunc(printLog)
	errLog := testutil.StderrOutputForFunc(printLog)

	if strings.Contains(infoLog, "DEBUG") || strings.Contains(infoLog, "INFO") {
		// Notice Log Level will not contain  DEBUG and  INFO logs
		t.Errorf("TestLogger_LevelDebug Failed!")
	}

	assertMessageInJSONLog(t, errLog, "Test Error Log")
}

func TestLogger_LevelWarn(t *testing.T) {
	printLog := func() {
		logger := NewLogger(WARN)
		logger.Debug("Test Debug Log")
		logger.Info("Test Info Log")
		logger.Notice("Test Notice Log")
		logger.Warn("Test Warn Log")
		logger.Error("Test Error Log")
	}

	infoLog := testutil.StdoutOutputForFunc(printLog)
	errLog := testutil.StderrOutputForFunc(printLog)

	if strings.ContainsAny(infoLog, "NOTICE|INFO|DEBUG") && !strings.Contains(errLog, "ERROR") {
		// Warn Log Level will not contain  DEBUG,INFO, NOTICE logs
		t.Errorf("TestLogger_LevelDebug Failed!")
	}

	assertMessageInJSONLog(t, errLog, "Test Error Log")
}

func TestLogger_LevelFatal(t *testing.T) {
	printLog := func() {
		logger := NewLogger(FATAL)
		logger.Debugf("%s", "Test Debug Log")
		logger.Infof("%s", "Test Info Log")
		logger.Noticef("%s", "Test Notice Log")
		logger.Warnf("%s", "Test Warn Log")
		logger.Errorf("%s", "Test Error Log")
	}

	infoLog := testutil.StdoutOutputForFunc(printLog)
	errLog := testutil.StderrOutputForFunc(printLog)

	assert.Equal(t, "", infoLog, "TestLogger_LevelFatal Failed!")
	assert.Equal(t, "", errLog, "TestLogger_LevelFatal Failed")
}

func assertMessageInJSONLog(t *testing.T, logLine, expectation string) {
	var l logEntry
	_ = json.Unmarshal([]byte(logLine), &l)

	if l.Message != expectation {
		t.Errorf("Log mismatch. Expected: %s Got: %s", expectation, l.Message)
	}
}

func TestCheckIfTerminal(t *testing.T) {
	tests := []struct {
		desc       string
		writer     io.Writer
		isTerminal bool
	}{
		{"Terminal Writer", os.Stdout, term.IsTerminal(int(os.Stdout.Fd()))},
		{"Non-Terminal Writer", os.Stderr, term.IsTerminal(int(os.Stderr.Fd()))},
		{"Non-Terminal Writer (not *os.File)", &bytes.Buffer{}, false},
	}

	for i, tc := range tests {
		result := checkIfTerminal(tc.writer)

		assert.Equal(t, tc.isTerminal, result, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_NewSilentLoggerSTDOutput(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		l := NewFileLogger("")

		l.Info("Info Logs")
		l.Debug("Debug Logs")
		l.Notice("Notic Logs")
		l.Warn("Warn Logs")
		l.Infof("%v Logs", "Infof")
		l.Debugf("%v Logs", "Debugf")
		l.Noticef("%v Logs", "Noticef")
		l.Warnf("%v Logs", "warnf")
	})

	assert.Equal(t, "", logs)
}

type mockLog struct {
	msg string
}

func (m *mockLog) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "TEST "+m.msg)
}

func TestPrettyPrint(t *testing.T) {
	m := &mockLog{msg: "mock test log"}
	out := &bytes.Buffer{}
	l := &logger{isTerminal: true}

	// case PrettyPrint is implemented
	l.prettyPrint(logEntry{
		Level:   INFO,
		Message: m,
	}, out)

	outputLog := out.String()
	expOut := []string{"INFO", "[00:00:00]", "TEST mock test log"}

	for _, v := range expOut {
		assert.Contains(t, outputLog, v)
	}

	// case pretty print is not implemented
	out.Reset()

	l.prettyPrint(logEntry{
		Level:   DEBUG,
		Message: "test log for normal log",
	}, out)

	outputLog = out.String()
	expOut = []string{"DEBU", "[00:00:00]", "test log for normal log"}

	for _, v := range expOut {
		assert.Contains(t, outputLog, v)
	}
}

func TestDefaultFilterMaskingPII(t *testing.T) {
	testCases := []struct {
		name           string
		input          interface{}
		expected       string
		enableMasking  bool
		maskingFields  []string
		expectedOutput interface{}
	}{
		{
			name: "mask string fields",
			input: struct {
				Name     string `json:"name"`
				Email    string `json:"email"`
				Password string `json:"password"`
			}{
				Name:     "John Doe",
				Email:    "john.doe@example.com",
				Password: "secret123",
			},
			expected:      `{"name":"********","email":"********************","password":"**********"}`,
			enableMasking: true,
			maskingFields: []string{"name", "email", "password"},
			expectedOutput: struct {
				Name     string `json:"name"`
				Email    string `json:"email"`
				Password string `json:"password"`
			}{
				Name:     "********",
				Email:    "********************",
				Password: "**********",
			},
		},
		{
			name: "mask numeric fields",
			input: struct {
				PhoneNumber          int64   `json:"phoneNumber"`
				SocialSecurityNumber int     `json:"socialSecurityNumber"`
				CreditCardNumber     uint64  `json:"creditCardNumber"`
				DateOfBirth          int     `json:"dateOfBirth"`
				BiometricData        float64 `json:"biometricData"`
				IPAddress            string  `json:"ipAddress"`
			}{
				PhoneNumber:          1234567890,
				SocialSecurityNumber: 123456789,
				CreditCardNumber:     1234567890123456,
				DateOfBirth:          19800101,
				BiometricData:        123.456,
				IPAddress:            "192.168.1.1",
			},
			expected:      `{"phoneNumber":0,"socialSecurityNumber":0,"creditCardNumber":0,"dateOfBirth":0,"biometricData":0,"ipAddress":"***********"}`,
			enableMasking: true,
			maskingFields: []string{"phoneNumber", "socialSecurityNumber", "creditCardNumber", "dateOfBirth", "biometricData", "ipAddress"},
			expectedOutput: struct {
				PhoneNumber          int64   `json:"phoneNumber"`
				SocialSecurityNumber int     `json:"socialSecurityNumber"`
				CreditCardNumber     uint64  `json:"creditCardNumber"`
				DateOfBirth          int     `json:"dateOfBirth"`
				BiometricData        float64 `json:"biometricData"`
				IPAddress            string  `json:"ipAddress"`
			}{
				PhoneNumber:          0,
				SocialSecurityNumber: 0,
				CreditCardNumber:     0,
				DateOfBirth:          0,
				BiometricData:        0,
				IPAddress:            "***********",
			},
		},
		{
			name:           "masking disabled",
			input:          struct{ Name string }{Name: "John Doe"},
			expected:       `{"Name":"John Doe"}`,
			enableMasking:  false,
			maskingFields:  []string{"name"},
			expectedOutput: struct{ Name string }{Name: "John Doe"},
		},
		{
			name: "mask fields in nested struct",
			input: struct {
				Name    string `json:"name"`
				Email   string `json:"email"`
				Address struct {
					Street string `json:"street"`
					City   string `json:"city"`
					Zip    int    `json:"zip"`
				} `json:"address"`
				CreditCard struct {
					Number string `json:"number"`
					CVV    int    `json:"cvv"`
				} `json:"creditCard"`
			}{
				Name:  "John Doe",
				Email: "john.doe@example.com",
				Address: struct {
					Street string `json:"street"`
					City   string `json:"city"`
					Zip    int    `json:"zip"`
				}{
					Street: "123 Main St",
					City:   "Anytown",
					Zip:    12345,
				},
				CreditCard: struct {
					Number string `json:"number"`
					CVV    int    `json:"cvv"`
				}{
					Number: "1234567890123456",
					CVV:    123,
				},
			},
			expected:      `{"name":"********","email":"********************","address":{"street":"***********","city":"*******","zip":12345},"creditCard":{"number":"****************","cvv":0}}`,
			enableMasking: true,
			maskingFields: []string{"name", "email", "street", "number", "cvv", "city"},
			expectedOutput: struct {
				Name    string `json:"name"`
				Email   string `json:"email"`
				Address struct {
					Street string `json:"street"`
					City   string `json:"city"`
					Zip    int    `json:"zip"`
				} `json:"address"`
				CreditCard struct {
					Number string `json:"number"`
					CVV    int    `json:"cvv"`
				} `json:"creditCard"`
			}{
				Name:  "********",
				Email: "********************",
				Address: struct {
					Street string `json:"street"`
					City   string `json:"city"`
					Zip    int    `json:"zip"`
				}{
					Street: "***********",
					City:   "*******",
					Zip:    12345,
				},
				CreditCard: struct {
					Number string `json:"number"`
					CVV    int    `json:"cvv"`
				}{
					Number: "****************",
					CVV:    0,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filter := &DefaultFilter{
				MaskFields:    tc.maskingFields,
				EnableMasking: tc.enableMasking,
			}

			filteredMessage := filter.Filter(tc.input)

			maskedMessage, err := json.Marshal(filteredMessage)
			if err != nil {
				t.Errorf("Failed to marshal masked message: %v", err)
				return
			}

			if string(maskedMessage) != tc.expected {
				t.Errorf("Expected %s, but got %s", tc.expected, maskedMessage)
			}

			if !reflect.DeepEqual(filteredMessage, tc.expectedOutput) {
				t.Errorf("Expected output %v, but got %v", tc.expectedOutput, filteredMessage)
			}
		})
	}
}
