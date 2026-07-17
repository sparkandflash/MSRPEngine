package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

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
			fmt.Println("debug: heartrate: 0.9 (placeholder value).")
		} else if input == "exit" || input == "quit" {
			fmt.Println("lyra: goodbye!")
			break
		} else {
			ctx := context.Background()
			response, err := resp.Respond(ctx, input)
			if err != nil {
				fmt.Printf("lyra: error: failed to generate response: %v\n", err)
			} else {
				fmt.Printf("lyra: %s\n", response)
			}
		}
	}
}

