package prompts

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed responder.txt
var rawResponderPrompt string

//go:embed reactor.txt
var rawReactorPrompt string

//go:embed personality.txt
var rawPersonalityPrompt string

// GetResponderPrompt returns the responder prompt combined with the personality prompt if defined.
func GetResponderPrompt() string {
	pers := strings.TrimSpace(rawPersonalityPrompt)
	if pers == "" {
		return strings.TrimSpace(rawResponderPrompt)
	}
	return fmt.Sprintf("%s\n\nPersonality guidelines:\n%s", strings.TrimSpace(rawResponderPrompt), pers)
}

// GetReactorPrompt returns the reactor prompt combined with the personality prompt if defined.
func GetReactorPrompt() string {
	pers := strings.TrimSpace(rawPersonalityPrompt)
	if pers == "" {
		return strings.TrimSpace(rawReactorPrompt)
	}
	return fmt.Sprintf("%s\n\nPersonality guidelines:\n%s", strings.TrimSpace(rawReactorPrompt), pers)
}
