package interfaceUI

import (
	"msrpe-vron-go/src/contextManager"
	"msrpe-vron-go/src/instanceManager"
	"msrpe-vron-go/src/ruleEngine"
	"msrpe-vron-go/src/utils"
)

// AppCore is the central dependency injection container that wires the
// Interface, Rule Engine, and Context Manager together, mirroring the V2 pattern.
type AppCore struct {
	Manager    *instanceManager.Manager
	RuleEngine *ruleEngine.ReflexDispatcher
	Context    *contextManager.ContextManager
}

// NewAppCore initializes and wires all subsystems.
func NewAppCore() (*AppCore, error) {
	utils.LogInfo("Wiring AppCore Subsystems...")

	// 1. Initialize Context Manager (Memory / Disk)
	ctxMgr, err := contextManager.NewContextManager()
	if err != nil {
		return nil, err
	}

	// 2. Initialize Instance Manager (Energy / Scheduler)
	instMgr := instanceManager.NewManager()

	// 3. Initialize Rule Engine (Reflexes)
	ruleEng := &ruleEngine.ReflexDispatcher{
		Manager: instMgr,
	}

	return &AppCore{
		Manager:    instMgr,
		RuleEngine: ruleEng,
		Context:    ctxMgr,
	}, nil
}
