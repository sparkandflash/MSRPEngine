package providers

import (
	"fmt"
	"msrpe-vron-go/src/vron"
)

// buildUserPrompt constructs the formatted user-turn string from a VRonContext.
// This is the exact payload the LLM sees as its "current situation."
func buildUserPrompt(ctx vron.VRonContext) string {
	return fmt.Sprintf(`--- CURRENT STATE ---
Energy Level: %d / %d
Consumption Rate: %d / tick
Active VRons: %d
Thread Depth: %d
Thread Cost: %d

--- SHORT-TERM MEMORY (Interface History) ---
%s

--- LONG-TERM MEMORY (Recalled Episodes) ---
%s

--- PASSED CONTEXT (From Parent VRon) ---
%s

Decide your action.`,
		ctx.EnergyLevel,
		ctx.MaxEnergy,
		ctx.ConsumptionRate,
		ctx.ActiveVRons,
		ctx.ThreadDepth,
		ctx.ThreadCost,
		ctx.STM,
		ctx.LTM,
		ctx.PassedContext,
	)
}
