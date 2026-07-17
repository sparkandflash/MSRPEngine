package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"lyra/consolidator"
	"lyra/responder"
)

// Run starts the interactive chat interface for Lyra.
func Run() {
	// Initialize the responder agent from environment configuration
	resp, err := responder.NewResponderFromEnv()
	if err != nil {
		fmt.Printf("system error: failed to initialize responder: %v\n", err)
		os.Exit(1)
	}

	// Initialize conversation history consolidator
	maxWorkingMemoryChars := 1500
	if limitStr := os.Getenv("LYRA_MAX_WORKING_MEMORY_CHARS"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && limit > 0 {
			maxWorkingMemoryChars = limit
		}
	}

	stm := consolidator.NewSTMmanager(maxWorkingMemoryChars)

	historyMgr, err := consolidator.NewHistoryManager()
	if err != nil {
		fmt.Printf("system error: failed to initialize history manager: %v\n", err)
		os.Exit(1)
	}

	heartRate := 0.35

	fmt.Println("lyra: hello, nice to meet you.")
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("user: ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if input == ">>debug" {
			fmt.Printf("debug: heartrate: %.2f (placeholder value).\n", heartRate)
		} else if strings.HasPrefix(input, ">>heartrate ") {
			valStr := strings.TrimSpace(strings.TrimPrefix(input, ">>heartrate "))
			var val float64
			_, err := fmt.Sscanf(valStr, "%f", &val)
			if err != nil || val < 0.1 || val > 0.9 {
				fmt.Println("debug: error: heartrate must be a float between 0.1 and 0.9.")
			} else {
				heartRate = val
				fmt.Printf("debug: heartrate updated to %.2f.\n", heartRate)
			}
		} else if input == "exit" || input == "quit" {
			fmt.Println("lyra: goodbye!")
			break
		} else {
			// Save user message to short-term memory (STM) and long-term history
			_ = historyMgr.Save("user", input)
			stm.Update("user", input)

			ctx := context.Background()
			response, err := resp.Respond(ctx, input, heartRate, stm.Get())
			if err != nil {
				fmt.Printf("lyra: error: failed to generate response: %v\n", err)
			} else {
				// Save assistant response to STM and history
				_ = historyMgr.Save("assistant", response)
				stm.Update("assistant", response)
				fmt.Printf("lyra: %s\n", response)
			}
		}
	}
}

