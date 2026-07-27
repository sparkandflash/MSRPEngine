package vron

import (
	"context"
	"fmt"
)

// Method represents the deterministic execution type assigned to a VRon.
type Method string

const (
	MethodRespond      Method = "respond"
	MethodConsolidate  Method = "consolidate"
	MethodQueryMemory  Method = "query_memory"
	MethodTest         Method = "test"
	MethodReact        Method = "react"
	MethodPlan         Method = "plan"
	MethodPromptUser   Method = "prompt_user"
	MethodContextSwap  Method = "context_swap"
	MethodUpdateMemory Method = "update_memory"
)

// VRonStatus represents the lifecycle state of a biological VRon cell.
type VRonStatus string

const (
	StatusNew        VRonStatus = "new"
	StatusActive     VRonStatus = "active"
	StatusWaiting    VRonStatus = "waiting"
	StatusIdle       VRonStatus = "idle"
	StatusTerminated VRonStatus = "terminated"
)

// MindScores represents the 5 core biological mindstate scores (MA:UA:SE:OX:CO).
type MindScores struct {
	MA float64 // Model Attention (0.00 - 1.00)
	UA float64 // User Attention / Uncertainty (0.00 - 1.00)
	SE float64 // Serotonin / Satisfaction (-1.00 - +1.00)
	OX float64 // Oxytocin / Social Connection (0.00 - 1.00)
	CO float64 // Cortisol / Stress (0.00 - 1.00)
}

func (ms MindScores) String() string {
	return fmt.Sprintf("%.2f:%.2f:%.2f:%.2f:%.2f", ms.MA, ms.UA, ms.SE, ms.OX, ms.CO)
}

// VRonContext encapsulates the I/O scopes for a VRon.
type VRonContext struct {
	STM             string     // Short-Term Memory (Interface History)
	LTM             string     // Long-Term Memory (Relevant Episodes from graph)
	MindState       string     // Formatted MA:UA:SE:OX:CO mind scores string
	EnergyLevel     int        // Current Global Energy
	SerotoninLevel  int        // Biological drive (-100 to 100)
	MaxEnergy       int        // Maximum capacity of the Global Energy pool
	ConsumptionRate int        // Rate at which energy is currently being drained
	PassedContext   string     // Context passed down from prior VRon or scheduler
	Method          Method     // Assigned deterministic method
	Status          VRonStatus // Current VRon lifecycle status
	Goal            string     // Legacy alias / Goal description (same as Method string)

	ThreadCost  int // Cumulative energy cost of the current VRon chain/thread
	ThreadDepth int // Number of VRons deep in the current chain
	ActiveVRons int // Total number of running VRon instances system-wide
}

// VRonOutput represents the structured JSON decision output by the LLM.
type VRonOutput struct {
	Response       string   `json:"response,omitempty"`
	Confidence     int      `json:"confidence,omitempty"`
	NeedMemory     bool     `json:"need_memory,omitempty"`
	MemoryQuery    string   `json:"memory_query,omitempty"`
	NeedTest       bool     `json:"need_test,omitempty"`
	TestQuery      string   `json:"test_query,omitempty"`
	Facts          []string `json:"facts,omitempty"`
	Result         string   `json:"result,omitempty"` // PASS, FAIL, CONFLICT, UNKNOWN
	Reasoning      string   `json:"reasoning,omitempty"`
	Observation    string   `json:"observation,omitempty"`
	SerotoninDelta int      `json:"serotonin_delta,omitempty"`
	Tasks          []string `json:"tasks,omitempty"`
	Message        string   `json:"message,omitempty"`
	Query          string   `json:"query,omitempty"`  // Legacy / payload query string
	Action         string   `json:"action,omitempty"` // Legacy payload action
}

// VRon defines the ephemeral reasoning cell.
type VRon interface {
	Execute(ctx context.Context, input VRonContext) (*VRonOutput, error)
}
