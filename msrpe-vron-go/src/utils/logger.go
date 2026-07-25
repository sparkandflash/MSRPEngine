package utils

import (
	"fmt"
	"log"
	"os"
)

// DebugMode controls whether verbose CLI logging is printed.
var DebugMode bool

var debugLogger = log.New(os.Stdout, "\033[36m[DEBUG]\033[0m ", log.Ltime)

// LogDebug prints detailed traces to the CLI console if the -debug flag is provided.
func LogDebug(format string, v ...interface{}) {
	if DebugMode {
		debugLogger.Printf(format, v...)
	}
}

// LogInfo prints standard operational info regardless of debug mode.
func LogInfo(format string, v ...interface{}) {
	fmt.Printf(format+"\n", v...)
}
