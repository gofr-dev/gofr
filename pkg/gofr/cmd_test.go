package gofr

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_Run_SuccessCallRegisteredArgument(t *testing.T) {
	os.Args = []string{"", "log"}

	c := cmd{}

	c.addRoute("log",
		func(c *Context) (interface{}, error) {
			c.Logger.Info("handler called")
			return nil, nil
		},
		AddDescription("Logs a message"),
		AddHelp("Custom helper documentation for log command"),
	)

	logs := testutil.StdoutOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile(".env", logging.NewMockLogger(logging.DEBUG))))
	})

	assert.Contains(t, logs, "handler called")
}

func Test_Run_SuccessSkipEmptySpaceAndMatchCommandWithSpace(t *testing.T) {
	os.Args = []string{"", "", " ", "log"}

	c := cmd{}

	c.addRoute("log",
		func(c *Context) (interface{}, error) {
			c.Logger.Info("handler called")
			return nil, nil
		},
		AddDescription("Logs a message"),
	)

	logs := testutil.StdoutOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", logging.NewMockLogger(logging.DEBUG))))
	})

	assert.Contains(t, logs, "handler called")
}

func Test_Run_SuccessCommandWithMultipleParameters(t *testing.T) {
	os.Args = []string{"", "log", "-param=value", "-b", "-c"}

	c := cmd{}

	c.addRoute("log",
		func(c *Context) (interface{}, error) {
			assert.Equal(t, "value", c.Request.Param("param"))
			assert.Equal(t, "true", c.Request.Param("b"))
			c.Logger.Info("handler called")

			return nil, nil
		},
		AddDescription("Logs a message with parameters"),
	)

	logs := testutil.StdoutOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", logging.NewMockLogger(logging.DEBUG))))
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

		c.addRoute(tc.args[1],
			func(c *Context) (interface{}, error) {
				c.Logger.Info("handler called")
				return nil, nil
			},
			AddDescription("Handles special character commands"),
		)

		logs := testutil.StdoutOutputForFunc(func() {
			c.Run(container.NewContainer(config.NewEnvFile("", logging.NewMockLogger(logging.DEBUG))))
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

		c.addRoute(tc.args[1],
			func(c *Context) (interface{}, error) {
				c.Logger.Info("handler called")
				return nil, nil
			},
			AddDescription("Handles special character commands"),
		)

		logs := testutil.StderrOutputForFunc(func() {
			c.Run(container.NewContainer(config.NewEnvFile("", logging.NewMockLogger(logging.DEBUG))))
		})

		assert.NotContains(t, logs, "handler called", "TEST[%d] Failed.\n %s", i, tc.desc)
		assert.Contains(t, logs, "No Command Found!", "TEST[%d] Failed.\n %s", i, tc.desc)
	}
}

func Test_Run_ErrorParamNotReadWithoutHyphen(t *testing.T) {
	os.Args = []string{"", "log", "hello=world"}

	c := cmd{}

	c.addRoute("log",
		func(c *Context) (interface{}, error) {
			assert.Equal(t, "", c.Request.Param("hello"))
			c.Logger.Info("handler called")

			return nil, nil
		},
		AddDescription("Logs a message"),
	)

	logs := testutil.StdoutOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", logging.NewMockLogger(logging.DEBUG))))
	})

	assert.Contains(t, logs, "handler called")
}

func Test_Run_ErrorNotARegisteredCommand(t *testing.T) {
	os.Args = []string{"", "log"}

	c := cmd{}

	logs := testutil.StderrOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", logging.NewMockLogger(logging.DEBUG))))
	})

	assert.Contains(t, logs, "No Command Found!")
}

func Test_Run_ErrorWhenOnlyParamAreGiven(t *testing.T) {
	os.Args = []string{"", "-route"}

	c := cmd{}

	c.addRoute("-route",
		func(c *Context) (interface{}, error) {
			c.Logger.Info("handler called of route -route")
			return nil, nil
		},
		AddDescription("Route with hyphen"),
	)

	logs := testutil.StderrOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", logging.NewMockLogger(logging.DEBUG))))
	})

	assert.Contains(t, logs, "No Command Found!")
	assert.NotContains(t, logs, "handler called of route -route")
}

func Test_Run_ErrorRouteRegisteredButNilHandler(t *testing.T) {
	os.Args = []string{"", "route"}

	c := cmd{}

	c.addRoute("route",
		nil,
		AddDescription("Nil handler route"),
	)

	logs := testutil.StderrOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", logging.NewMockLogger(logging.DEBUG))))
	})

	assert.Contains(t, logs, "No Command Found!")
}

func Test_Run_ErrorNoArgumentGiven(t *testing.T) {
	errlog := ""
	os.Args = []string{""}

	c := cmd{}

	out := testutil.StdoutOutputForFunc(func() {
		errlog = testutil.StderrOutputForFunc(func() {
			c.Run(container.NewContainer(config.NewEnvFile("", logging.NewMockLogger(logging.DEBUG))))
		})
	})

	assert.Contains(t, errlog, "No Command Found!")
	assert.Contains(t, out, "Available commands:")
}

func Test_Run_SuccessCallInvalidHyphens(t *testing.T) {
	os.Args = []string{"", "log", "-param=value", "-b", "-"}

	c := cmd{}

	c.addRoute("log",
		func(c *Context) (interface{}, error) {
			c.Logger.Info("handler called")
			return nil, nil
		},
		AddDescription("Logs a message"),
	)

	logs := testutil.StdoutOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", logging.NewMockLogger(logging.DEBUG))))
	})

	assert.Contains(t, logs, "handler called")
}

func Test_Run_HelpCommand(t *testing.T) {
	os.Args = []string{"", "-h"}

	c := cmd{}

	c.addRoute("log",
		func(c *Context) (interface{}, error) {
			c.Logger.Info("handler called")
			return nil, nil
		},
		AddDescription("Logs a message"),
		AddHelp("logging messages to the terminal"),
	)

	logs := testutil.StdoutOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", logging.NewMockLogger(logging.DEBUG))))
	})

	assert.Contains(t, logs, "Available commands:")
	assert.Contains(t, logs, "Description: Logs a message")
	assert.Contains(t, logs, "logging messages to the terminal")
}

func Test_Run_HelpCommandLong(t *testing.T) {
	os.Args = []string{"", "--help"}

	c := cmd{}

	c.addRoute("log",
		func(c *Context) (interface{}, error) {
			c.Logger.Info("handler called")
			return nil, nil
		},
		AddDescription("Logs a message"),
		AddHelp("logging messages to the terminal"),
	)

	logs := testutil.StdoutOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewEnvFile("", logging.NewMockLogger(logging.DEBUG))))
	})

	assert.Contains(t, logs, "Available commands:")
	assert.Contains(t, logs, "Description: Logs a message")
	assert.Contains(t, logs, "logging messages to the terminal")
}

func Test_Run_UnknownCommandShowsHelp(t *testing.T) {
	errLogs := ""
	os.Args = []string{"", "unknown"}

	c := cmd{}

	c.addRoute("log",
		func(c *Context) (interface{}, error) {
			c.Logger.Info("handler called")
			return nil, nil
		},
		AddDescription("Logs a message"),
		AddHelp("logging messages to the terminal"),
	)

	logs := testutil.StdoutOutputForFunc(func() {
		errLogs = testutil.StderrOutputForFunc(func() {
			c.Run(container.NewContainer(config.NewEnvFile("", logging.NewMockLogger(logging.DEBUG))))
		})
	})

	assert.Contains(t, errLogs, "No Command Found!")
	assert.Contains(t, logs, "Available commands:")
	assert.Contains(t, logs, "Description: Logs a message")
	assert.Contains(t, logs, "logging messages to the terminal")
}

func Test_Run_handler_help(t *testing.T) {
	var old []string

	args := []string{"", "hello", "--help"}
	old, os.Args = os.Args, args

	t.Cleanup(func() {
		os.Args = old
	})

	c := cmd{}

	c.addRoute("hello", func(_ *Context) (interface{}, error) {
		return "Hello", nil
	}, AddHelp("this a helper string for hello sub command"))

	out := testutil.StdoutOutputForFunc(func() {
		c.Run(container.NewContainer(config.NewMockConfig(map[string]string{})))
	})

	// check that only help for the hello subcommand is printed
	assert.Equal(t, "this a helper string for hello sub command\n", out)
}
