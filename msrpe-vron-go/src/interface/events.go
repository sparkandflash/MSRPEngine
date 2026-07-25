package interfaceUI

import (
	"bufio"
	"context"
	"os"
	"strings"
)

// RunLoop starts the interactive terminal session.
func (app *AppCore) RunLoop(ctx context.Context) {
	scanner := bufio.NewScanner(os.Stdin)

	PrintSystemAlert("Engine Ready. Type a message below.")

	for {
		PrintUserPrompt()

		// Read CLI input
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())

		if input == "" {
			continue
		}
		if strings.ToLower(input) == "exit" || strings.ToLower(input) == "quit" {
			PrintSystemAlert("Shutting down...")
			break
		}

		// 1. Log to Interface History (STM)
		err := app.Context.HistoryManager.Append("User", input)
		if err != nil {
			PrintSystemAlert("Warning: Failed to log history: " + err.Error())
		}

		// 2. Route to Rule Engine (Reflex)
		app.RuleEngine.OnUserMessage(input)
	}
}
