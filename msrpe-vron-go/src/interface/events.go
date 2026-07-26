package interfaceUI

import (
	"context"
	"strings"

	"github.com/chzyer/readline"
	"msrpe-vron-go/src/utils"
)

// RunLoop starts the interactive terminal session.
func (app *AppCore) RunLoop(ctx context.Context) {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "\033[97m>>\033[0m ",
		HistoryFile:     "Context/cli_history.tmp",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		PrintSystemAlert("Failed to initialize readline. Falling back to simple scan.")
		return
	}
	defer rl.Close()

	// Redirect all standard printing to readline so it doesn't interrupt typing
	utils.SetOutput(rl.Stdout())

	PrintSystemAlert("Engine Ready. Type a message below.")

	for {
		// Read CLI input
		line, err := rl.Readline()
		if err != nil { // EOF or interrupt
			break
		}

		input := strings.TrimSpace(line)

		if input == "" {
			continue
		}
		if strings.ToLower(input) == "exit" || strings.ToLower(input) == "quit" {
			PrintSystemAlert("Shutting down...")
			break
		}

		// 1. Log to Interface History (STM)
		err = app.Context.HistoryManager.Append("User", input)
		if err != nil {
			PrintSystemAlert("Warning: Failed to log history: " + err.Error())
		}

		// 2. Route to Rule Engine (Reflex)
		app.RuleEngine.OnUserMessage(input)
	}
}
