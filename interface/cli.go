package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"lyra/consolidator"
	"lyra/reactor"
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

	// Initialize the reactor agent
	reactorAgent := reactor.NewReactorAgent()

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

	mindState := "0.90:0.30:0.50:0.70"

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
			fmt.Printf("debug: mindstate: %s\n", mindState)
		} else if strings.HasPrefix(input, ">>mindstate ") {
			valStr := strings.TrimSpace(strings.TrimPrefix(input, ">>mindstate "))
			var ma, ne, pe, ua float64
			_, err := fmt.Sscanf(valStr, "%f:%f:%f:%f", &ma, &ne, &pe, &ua)
			if err != nil || ma < 0.0 || ma > 1.0 || ne < 0.0 || ne > 1.0 || pe < 0.0 || pe > 1.0 || ua < 0.0 || ua > 1.0 {
				fmt.Println("debug: error: mindstate must be four floats (0.0 to 1.0) separated by colons (e.g. 0.9:0.3:0.5:0.7).")
			} else {
				mindState = fmt.Sprintf("%.2f:%.2f:%.2f:%.2f", ma, ne, pe, ua)
				fmt.Printf("debug: mindstate updated to %s.\n", mindState)
			}
		} else if input == "exit" || input == "quit" {
			fmt.Println("lyra: goodbye!")
			break
		} else {
			ctx := context.Background()

			// Save user message to short-term memory (STM) and long-term history
			_ = historyMgr.Save("user", input)
			stm.Update("user", input)

			// Invoke reactor agent to determine mindstate after user input
			if respState, err := reactorAgent.React(ctx, stm.Get()); err == nil {
				mindState = fmt.Sprintf("%.2f:%.2f:%.2f:%.2f", respState.ModelAttention, respState.NegativeEmotion, respState.PositiveEmotion, respState.UserAttention)
			}

			response, err := resp.Respond(ctx, input, mindState, stm.Get())
			if err != nil {
				fmt.Printf("lyra: error: failed to generate response: %v\n", err)
			} else {
				// Save assistant response to STM and history
				_ = historyMgr.Save("assistant", response)
				stm.Update("assistant", response)

				// Invoke reactor agent to determine mindstate after assistant response
				if respState, err := reactorAgent.React(ctx, stm.Get()); err == nil {
					mindState = fmt.Sprintf("%.2f:%.2f:%.2f:%.2f", respState.ModelAttention, respState.NegativeEmotion, respState.PositiveEmotion, respState.UserAttention)
				}

				fmt.Printf("lyra: %s\n", response)
			}
		}
	}
}

