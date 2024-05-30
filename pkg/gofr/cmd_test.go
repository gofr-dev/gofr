package gofr

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_Run_SuccessCallRegisteredArgument(t *testing.T) {
	os.Args = []string{"", "log"}

	c := cmd{
		defaultHelp: "Default helper documentation",
	}

	c.addRoute(
		route{
			pattern: "log",
			handler: func(c *Context) (interface{}, error) {
				c.Logger.Info("handler called")
				return nil, nil
			},
			description: "Logs a message",
			help:        "Custom helper documentation for log command",
		})

	logs := testutil.StdoutOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile(".env", testutil.NewMockLogger(testutil.DEBUGLOG))))
	})

	assert.Contains(t, logs, "handler called")
}

func Test_Run_SuccessSkipEmptySpaceAndMatchCommandWithSpace(t *testing.T) {
	os.Args = []string{"", "", " ", "log"}

	c := cmd{}

	c.addRoute(
		route{
			pattern: "log",
			handler: func(c *Context) (interface{}, error) {
				c.Logger.Info("handler called")
				return nil, nil
			},
			description: "Logs a message",
		})

	logs := testutil.StdoutOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", testutil.NewMockLogger(testutil.DEBUGLOG))))
	})

	assert.Contains(t, logs, "handler called")
}

func Test_Run_SuccessCommandWithMultipleParameters(t *testing.T) {
	os.Args = []string{"", "log", "-param=value", "-b", "-c"}

	c := cmd{}

	c.addRoute(
		route{
			pattern: "log",
			handler: func(c *Context) (interface{}, error) {
				assert.Equal(t, c.Request.Param("param"), "value")
				assert.Equal(t, c.Request.Param("b"), "true")
				c.Logger.Info("handler called")
				return nil, nil
			},
			description: "Logs a message with parameters",
		})

	logs := testutil.StdoutOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", testutil.NewMockLogger(testutil.DEBUGLOG))))
	})

	assert.Contains(t, logs, "handler called")
}

func Test_Run_SuccessRouteWithSpecialCharacters(t *testing.T) {
	testCases := []struct {
		desc string
		args []string
	}{
		{"special character !", []string{"", "command-with-special-characters!"}},
		{"special character @", []string{"", "command-with-special-characters@"}},
		{"special character #", []string{"", "command-with-special-characters#"}},
		{"special character %", []string{"", "command-with-special-characters%"}},
		{"special character &", []string{"", "command-with-special-characters&"}},
		{"special character *", []string{"", "command-with-special-characters*"}},
	}

	for i, tc := range testCases {
		os.Args = tc.args
		c := cmd{}

		c.addRoute(
			route{
				pattern: tc.args[1],
				handler: func(c *Context) (interface{}, error) {
					c.Logger.Info("handler called")
					return nil, nil
				},
				description: "Handles special character commands",
			})

		logs := testutil.StdoutOutputForFunc(func() {
			c.Run(container.NewContainer(config.NewEnvFile("", testutil.NewMockLogger(testutil.DEBUGLOG))))
		})

		assert.Contains(t, logs, "handler called", "TEST[%d] Failed.\n %s", i, tc.desc)
		assert.NotContains(t, logs, "No Command Found!", "TEST[%d] Failed.\n %s", i, tc.desc)
	}
}

func Test_Run_ErrorRouteWithSpecialCharacters(t *testing.T) {
	testCases := []struct {
		desc string
		args []string
	}{
		{"special character $", []string{"", "command-with-special-characters$"}},
		{"special character ^", []string{"", "command-with-special-characters^"}},
	}

	for i, tc := range testCases {
		os.Args = tc.args
		c := cmd{}

		c.addRoute(
			route{
				pattern: tc.args[1],
				handler: func(c *Context) (interface{}, error) {
					c.Logger.Info("handler called")
					return nil, nil
				},
				description: "Handles special character commands",
			})

		logs := testutil.StderrOutputForFunc(func() {
			c.Run(container.NewContainer(config.NewEnvFile("", testutil.NewMockLogger(testutil.DEBUGLOG))))
		})

		assert.NotContains(t, logs, "handler called", "TEST[%d] Failed.\n %s", i, tc.desc)
		assert.Contains(t, logs, "No Command Found!", "TEST[%d] Failed.\n %s", i, tc.desc)
	}
}

func Test_Run_ErrorParamNotReadWithoutHyphen(t *testing.T) {
	os.Args = []string{"", "log", "hello=world"}

	c := cmd{}

	c.addRoute(
		route{
			pattern: "log",
			handler: func(c *Context) (interface{}, error) {
				assert.Equal(t, c.Request.Param("hello"), "")
				c.Logger.Info("handler called")
				return nil, nil
			},
			description: "Logs a message",
		})

	logs := testutil.StdoutOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", testutil.NewMockLogger(testutil.DEBUGLOG))))
	})

	assert.Contains(t, logs, "handler called")
}

func Test_Run_ErrorNotARegisteredCommand(t *testing.T) {
	os.Args = []string{"", "log"}

	c := cmd{}

	logs := testutil.StderrOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", testutil.NewMockLogger(testutil.DEBUGLOG))))
	})

	assert.Contains(t, logs, "No Command Found!")
}

func Test_Run_ErrorWhenOnlyParamAreGiven(t *testing.T) {
	os.Args = []string{"", "-route"}

	c := cmd{}

	c.addRoute(
		route{
			pattern: "-route",
			handler: func(c *Context) (interface{}, error) {
				c.Logger.Info("handler called of route -route")
				return nil, nil
			},
			description: "Route with hyphen",
		})

	logs := testutil.StderrOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", testutil.NewMockLogger(testutil.DEBUGLOG))))
	})

	assert.Contains(t, logs, "No Command Found!")
	assert.NotContains(t, logs, "handler called of route -route")
}

func Test_Run_ErrorRouteRegisteredButNilHandler(t *testing.T) {
	os.Args = []string{"", "route"}

	c := cmd{}

	c.addRoute(
		route{
			pattern:     "route",
			handler:     nil,
			description: "Nil handler route",
		})

	logs := testutil.StderrOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", testutil.NewMockLogger(testutil.DEBUGLOG))))
	})

	assert.Contains(t, logs, "No Command Found!")
}

func Test_Run_ErrorNoArgumentGiven(t *testing.T) {
	os.Args = []string{""}

	c := cmd{}

	logs := testutil.StderrOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", testutil.NewMockLogger(testutil.DEBUGLOG))))
	})

	assert.Contains(t, logs, "Available commands:")
	assert.Contains(t, logs, "No Command Found!")
}

func Test_Run_SuccessCallInvalidHyphens(t *testing.T) {
	os.Args = []string{"", "log", "-param=value", "-b", "-"}

	c := cmd{}

	c.addRoute(
		route{
			pattern: "log",
			handler: func(c *Context) (interface{}, error) {
				c.Logger.Info("handler called")
				return nil, nil
			},
			description: "Logs a message",
		})

	logs := testutil.StdoutOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", testutil.NewMockLogger(testutil.DEBUGLOG))))
	})

	assert.Contains(t, logs, "handler called")
}

func Test_Run_HelpCommand(t *testing.T) {
	os.Args = []string{"", "-h"}

	c := cmd{}

	c.addRoute(
		route{
			pattern: "log",
			handler: func(c *Context) (interface{}, error) {
				c.Logger.Info("handler called")
				return nil, nil
			},
			description: "Logs a message",
		})

	logs := testutil.StdoutOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", testutil.NewMockLogger(testutil.DEBUGLOG))))
	})

	assert.Contains(t, logs, "Available commands:")
	assert.Contains(t, logs, "log: Logs a message")
}

func Test_Run_HelpCommandLong(t *testing.T) {
	os.Args = []string{"", "--help"}

	c := cmd{}

	c.addRoute(
		route{
			pattern: "log",
			handler: func(c *Context) (interface{}, error) {
				c.Logger.Info("handler called")
				return nil, nil
			},
			description: "Logs a message",
		})

	logs := testutil.StdoutOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", testutil.NewMockLogger(testutil.DEBUGLOG))))
	})

	assert.Contains(t, logs, "Available commands:")
	assert.Contains(t, logs, "log: Logs a message")
}

func Test_Run_UnknownCommandShowsHelp(t *testing.T) {
	os.Args = []string{"", "unknown"}

	c := cmd{}

	c.addRoute(
		route{
			pattern: "log",
			handler: func(c *Context) (interface{}, error) {
				c.Logger.Info("handler called")
				return nil, nil
			},
			description: "Logs a message",
		})

	logs := testutil.StderrOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", testutil.NewMockLogger(testutil.DEBUGLOG))))
	})

	assert.Contains(t, logs, "Unknown command: unknown")
	assert.Contains(t, logs, "Available commands:")
	assert.Contains(t, logs, "log: Logs a message")
}
