// Package assert provides the functionality to check the correctness of the output of a command line function
package assert

import (
	"io"
	"log"
	"os"
	"strings"
	"testing"
)

func getOutput(main func(), command string) string {
	tmpfile, _ := os.CreateTemp("", "fake-stdout.*")

	defer os.Remove(tmpfile.Name())

	os.Stdout = tmpfile
	os.Args = strings.Split(command, " ")

	log.SetOutput(io.Discard)

	main()

	outputBytes, _ := os.ReadFile(tmpfile.Name())

	output := strings.TrimSpace(string(outputBytes))

	return output
}

// CMDOutputContains is a testing utility that verifies whether the output produced
// by the main function executed with a specific command contains an expected string.
func CMDOutputContains(t *testing.T, main func(), command, expected string) {
	output := getOutput(main, command)

	if !strings.Contains(output, expected) {
		t.Errorf("Expected output: %s Got: %s", expected, output)

		return
	}

	t.Logf("Test passed for '%s'", command)
}
