package interfaceUI

import (
	"fmt"
	"strings"
)

// PrintUserPrompt prints the standard V2 '>> ' prompt.
func PrintUserPrompt() {
	fmt.Print("\n\033[32m>>\033[0m ")
}

// PrintVRonResponse prints a message originating from a VRon or the System.
func PrintVRonResponse(sender string, message string) {
	fmt.Printf("\n\033[35m[%s]\033[0m %s\n", strings.ToUpper(sender), message)
}

// PrintSystemAlert prints a system-level alert.
func PrintSystemAlert(message string) {
	fmt.Printf("\n\033[33m[SYSTEM ALERT]\033[0m %s\n", message)
}
