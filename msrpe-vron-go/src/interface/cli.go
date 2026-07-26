package interfaceUI

import (
	"fmt"
	"strings"

	"msrpe-vron-go/src/utils"
)

// PrintUserPrompt is now handled by Readline configuration, but we keep this signature just in case.
func PrintUserPrompt() {}

// PrintVRonResponse prints a message originating from a VRon or the System.
func PrintVRonResponse(sender string, message string) {
	fmt.Fprintf(utils.Output, "\033[34m[%s] %s\033[0m\n", strings.ToUpper(sender), message)
}

// PrintSystemAlert prints a system-level alert.
func PrintSystemAlert(message string) {
	fmt.Fprintf(utils.Output, "\033[90m[SYSTEM ALERT] %s\033[0m\n", message)
}
