package vron

import (
	"context"
)

// VRonContext encapsulates the 5 explicit I/O scopes for a VRon.
// Note: Emotional mind states (MA/UA/SE) are intentionally omitted to force
// behavior to emerge purely from graph logic and energy constraints.
type VRonContext struct {
	STM             string // Short-Term Memory (Interface History)
	LTM             string // Long-Term Memory (Relevant Episodes from graph)
	EnergyLevel     int    // Current Global Energy (0-100)
	ConsumptionRate int    // Rate at which energy is currently being drained
	PassedContext   string // Context specifically passed down from a parent VRon

	ThreadCost      int    // Cumulative energy cost of the current VRon chain/thread
	ThreadDepth     int    // Number of VRons deep in the current chain (vron1->vron2->vron3 = 3)
	ActiveVRons     int    // Total number of alive/running VRon instances system-wide
}

// VRonOutput represents the structured JSON decision output by the LLM.
// It maps directly to OpenAI/Gemini Native Structured Output JSON Schema.
type VRonOutput struct {
	Action string `json:"action"` // "respond", "spawn_child", "update_memory", "test_result"
	Goal   string `json:"goal"`   // The goal assigned to the child VRon, or self
	Query  string `json:"query"`  // The specific query, test, or response payload
}

// VRon defines the ephemeral reasoning cell.
type VRon interface {
	// Execute runs the VRon with the given context. It returns the structured output
	// representing the LLM's decision.
	Execute(ctx context.Context, input VRonContext) (*VRonOutput, error)
}
