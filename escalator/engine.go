package escalator

import (
	"time"
)

// EventType represents a declarative action decided by the Rule Engine.
type EventType string

const (
	EventNothing          EventType = "NOTHING"
	EventConsolidate      EventType = "CONSOLIDATE"
	EventReflect          EventType = "REFLECT"
	EventProactiveMessage EventType = "PROACTIVE_MESSAGE"
	EventIntrospect       EventType = "INTROSPECT"
)

// RuleEngine maintains internal state (like Heartrate) and deterministically
// evaluates rules to emit events.
type RuleEngine struct {
	// Internal State
	Heartrate              float64
	MovingAverageUserDelay time.Duration

	// Timestamps
	LastUserMessage      time.Time
	LastAssistantMessage time.Time
	LastConsolidation    time.Time
	LastReflection       time.Time
	LastIntrospection    time.Time
	LastProactiveMessage time.Time
}

// NewRuleEngine initializes a new engine with resting defaults.
func NewRuleEngine() *RuleEngine {
	now := time.Now()
	return &RuleEngine{
		Heartrate:              70.0,
		MovingAverageUserDelay: 10 * time.Second, // Default starting assumption
		LastUserMessage:        now,
		LastAssistantMessage:   now,
		LastConsolidation:      now,
		LastReflection:         now,
		LastIntrospection:      now,
		LastProactiveMessage:   now,
	}
}
