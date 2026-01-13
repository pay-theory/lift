package stdio

import (
	"fmt"
	"os"
)

// Stdoutf writes formatted output to stdout, ignoring write errors.
func Stdoutf(format string, args ...any) {
	if _, err := fmt.Fprintf(os.Stdout, format, args...); err != nil {
		return
	}
}

// Stdoutln writes a line to stdout, ignoring write errors.
func Stdoutln(args ...any) {
	if _, err := fmt.Fprintln(os.Stdout, args...); err != nil {
		return
	}
}

// Stdout writes to stdout, ignoring write errors.
func Stdout(args ...any) {
	if _, err := fmt.Fprint(os.Stdout, args...); err != nil {
		return
	}
}

// Stderrf writes formatted output to stderr, ignoring write errors.
func Stderrf(format string, args ...any) {
	if _, err := fmt.Fprintf(os.Stderr, format, args...); err != nil {
		return
	}
}

// Stderrln writes a line to stderr, ignoring write errors.
func Stderrln(args ...any) {
	if _, err := fmt.Fprintln(os.Stderr, args...); err != nil {
		return
	}
}

// Stderr writes to stderr, ignoring write errors.
func Stderr(args ...any) {
	if _, err := fmt.Fprint(os.Stderr, args...); err != nil {
		return
	}
}
