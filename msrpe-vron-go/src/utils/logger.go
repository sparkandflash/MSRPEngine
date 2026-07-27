package utils

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sync"
	"time"
)

// DebugMode controls whether verbose CLI logging is printed.
var DebugMode bool

// Output is the target for all CLI printing (defaults to os.Stdout)
var Output io.Writer = os.Stdout

var (
	logFile   *os.File
	fileMu    sync.Mutex
	ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\033\[[0-9;]*[a-zA-Z]`)
)

func init() {
	f, err := os.OpenFile("logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		logFile = f
	}
}

// SetOutput allows redirecting logs (e.g., to readline.Stdout() for concurrency safety).
func SetOutput(w io.Writer) {
	Output = w
}

// WriteLogFile appends a cleaned timestamped message to logs.txt.
func WriteLogFile(prefix string, msg string) {
	if logFile == nil {
		return
	}
	fileMu.Lock()
	defer fileMu.Unlock()

	cleanMsg := ansiRegex.ReplaceAllString(msg, "")
	timestamp := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s %s\n", timestamp, prefix, cleanMsg)
	_, _ = logFile.WriteString(line)
	_ = logFile.Sync()
}

// LogDebug prints detailed traces to the CLI console if the -debug flag is provided, and writes to logs.txt.
func LogDebug(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if DebugMode {
		fmt.Fprintf(Output, "\033[90m[DEBUG] %s\033[0m\n", msg)
		if f, ok := Output.(*os.File); ok {
			_ = f.Sync()
		}
	}
	WriteLogFile("[DEBUG]", msg)
}

// LogInfo prints standard operational info regardless of debug mode, and writes to logs.txt.
func LogInfo(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	fmt.Fprintf(Output, "\033[90m%s\033[0m\n", msg)
	if f, ok := Output.(*os.File); ok {
		_ = f.Sync()
	}
	WriteLogFile("[INFO]", msg)
}
