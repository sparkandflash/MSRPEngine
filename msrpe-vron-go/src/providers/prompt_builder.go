package providers

import (
	"fmt"
	"msrpe-vron-go/src/vron"
)

// buildUserPrompt constructs the formatted user-turn string from a VRonContext.
// This is the exact payload the LLM sees as its current situation.
func buildUserPrompt(ctx vron.VRonContext) string {
	methodStr := string(ctx.Method)
	if methodStr == "" {
		methodStr = ctx.Goal
	}

	return fmt.Sprintf(`--- CURRENT STATE ---
Assigned Method: %s
Energy Level: %d / %d
Serotonin Level (SE): %d
Consumption Rate: %d / tick
Active VRons: %d
Thread Depth: %d
Thread Cost: %d

--- SHORT-TERM MEMORY (Interface History) ---
%s

--- LONG-TERM MEMORY (Recalled Episodes) ---
%s

--- PASSED CONTEXT (From Prior VRon / Scheduler) ---
%s

Execute assigned method '%s' strictly.`,
		methodStr,
		ctx.EnergyLevel,
		ctx.MaxEnergy,
		ctx.SerotoninLevel,
		ctx.ConsumptionRate,
		ctx.ActiveVRons,
		ctx.ThreadDepth,
		ctx.ThreadCost,
		ctx.STM,
		ctx.LTM,
		ctx.PassedContext,
		methodStr,
	)
}
