package ruleEngine

import (
	"fmt"
	"msrpe-vron-go/src/instanceManager"
	"msrpe-vron-go/src/vron"
)

// ReflexDispatcher acts as the absolute baseline nervous system.
// It does NOT plan or reason. It simply reacts to external interface events
// and wakes up the InstanceManager by spawning a Root VRon.
type ReflexDispatcher struct {
	Manager *instanceManager.Manager
}

// OnUserMessage is a reflex triggered when the interface receives a chat message.
func (r *ReflexDispatcher) OnUserMessage(message string) {
	fmt.Printf("[RuleEngine] Reflex Triggered: User Message Received.\n")

	// Assemble the initial context (STM)
	ctx := vron.VRonContext{
		STM:             fmt.Sprintf("User says: %s", message),
		LTM:             "", // Root VRon must spawn a child to retrieve this if needed
		EnergyLevel:     r.Manager.GlobalEnergy,
		ConsumptionRate: 0,
		PassedContext:   "",
	}

	// Spawn the Root VRon. Notice we pass an empty parentID because this is a root.
	err := r.Manager.Spawn("", ctx, nil)
	if err != nil {
		fmt.Printf("[RuleEngine] Reflex failed to spawn VRon: %v\n", err)
	}
}
