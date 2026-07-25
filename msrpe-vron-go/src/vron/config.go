package vron

// Character limits for VRon components
const (
	MaxVRonResponseLength = 2000
	MaxUserMessageLength  = 2000
)

// SystemMessage defines standard system-level protocols and formats.
type SystemMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// Pre-defined System Message constants for structural signaling
const (
	SysMsgRetrievalFailed = "[System: Context Retrieval Failed. You have zero memory of this topic.]"
	SysMsgGoalResolved    = "[System: Goal resolved.]"
	SysMsgFatigue         = "[System: Energy critically low. Network is fatigued.]"
)
