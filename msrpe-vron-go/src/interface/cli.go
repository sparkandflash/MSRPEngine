package interfaceUI

import (
	"fmt"

	"msrpe-vron-go/src/utils"
)

// PrintUserPrompt is now handled by Readline configuration.
func PrintUserPrompt() {}

// PrintVRonResponse prints a message originating from the Organism (e.g. [Lyra]).
func PrintVRonResponse(sender string, message string) {
	if sender == "" {
		sender = "Lyra"
	}
	fmt.Fprintf(utils.Output, "\033[34m[%s] %s\033[0m\n", sender, message)
	utils.WriteLogFile(fmt.Sprintf("[%s]", sender), message)
}

// PrintSystemAlert prints a system-level alert [system].
func PrintSystemAlert(message string) {
	fmt.Fprintf(utils.Output, "\033[90m[system] %s\033[0m\n", message)
	utils.WriteLogFile("[system]", message)
}
