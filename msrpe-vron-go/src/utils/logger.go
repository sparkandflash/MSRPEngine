package utils

import (
	"fmt"
	"io"
	"log"
	"os"
)

// DebugMode controls whether verbose CLI logging is printed.
var DebugMode bool

// Output is the target for all CLI printing (defaults to os.Stdout)
var Output io.Writer = os.Stdout

var debugLogger = log.New(Output, "\033[90m[DEBUG] ", log.Ltime)

// SetOutput allows redirecting logs (e.g., to readline.Stdout() for concurrency safety).
func SetOutput(w io.Writer) {
	Output = w
	debugLogger.SetOutput(w)
}

// LogDebug prints detailed traces to the CLI console if the -debug flag is provided.
func LogDebug(format string, v ...interface{}) {
	if DebugMode {
		msg := fmt.Sprintf(format, v...)
		debugLogger.Printf("%s\033[0m", msg)
	}
}

// LogInfo prints standard operational info regardless of debug mode.
func LogInfo(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	fmt.Fprintf(Output, "\033[90m%s\033[0m\n", msg)
}
