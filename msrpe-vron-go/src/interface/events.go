package interfaceUI

import (
	"context"
	"io"
	"os"
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

	// Watch context cancellation and close readline so blocking Readline() returns
	go func() {
		<-ctx.Done()
		rl.Close()
	}()

	// Use os.Stdout directly to prevent background VRon goroutine deadlocks on readline lock
	utils.SetOutput(os.Stdout)

	PrintSystemAlert("Engine Ready. Type a message below.")

	for {
		// Read CLI input
		line, err := rl.Readline()
		if err != nil { // EOF or interrupt or context cancel
			if err == io.EOF {
				// Prevent premature shutdown on piped/task stdin EOF; wait for context cancellation
				<-ctx.Done()
			}
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

		// 2. Route via Scheduler (or Reflex Engine fallback)
		if app.Scheduler != nil {
			if err := app.Scheduler.TriggerUserMessage(input); err != nil {
				PrintSystemAlert("Engine alert: " + err.Error())
				utils.LogInfo("[Scheduler] TriggerUserMessage error: %v", err)
			}
		} else {
			app.RuleEngine.OnUserMessage(input)
		}
	}
}
