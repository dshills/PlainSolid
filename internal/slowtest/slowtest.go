// Package slowtest keeps tests that take seconds or longer out of the default
// `go test` run. Such tests call Skip first and run only when the environment
// opts in, which CI and the qualification launcher do.
package slowtest

import (
	"os"
	"testing"
)

// Env names the environment variable that enables slow tests when set to "1".
const Env = "PLAINSOLID_SLOW"

// Enabled reports whether slow tests were requested.
func Enabled() bool { return os.Getenv(Env) == "1" }

// Skip skips the calling test unless slow tests were requested.
func Skip(t testing.TB) {
	t.Helper()
	if !Enabled() {
		t.Skip("slow test: set " + Env + "=1 to run")
	}
}
