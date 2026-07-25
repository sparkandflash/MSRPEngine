package interfaceUI

import (
	"context"

	"msrpe-vron-go/src/contextManager"
	"msrpe-vron-go/src/envconfig"
	"msrpe-vron-go/src/instanceManager"
	"msrpe-vron-go/src/ruleEngine"
	"msrpe-vron-go/src/utils"
	"msrpe-vron-go/src/vron"
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
	// 1. Load absolute capacities from .env via envconfig
	config := envconfig.Load()
	vron.MaxVRonResponseLength = config.MaxResponseChars
	vron.MaxUserMessageLength = config.MaxUserMessageChars

	utils.LogInfo("Wiring AppCore Subsystems...")

	// 2. Initialize Context Manager (Memory / Disk)
	// It will independently fetch the EmbeddingProvider
	ctxMgr, err := contextManager.NewContextManager()
	if err != nil {
		return nil, err
	}

	// 3. Initialize Rule Engine (Reflexes)
	ruleEng := ruleEngine.NewReflexDispatcher()

	// 4. Initialize Instance Manager (Engine Core)
	// It will independently fetch the InferenceProvider and validate it.
	instMgr, err := instanceManager.NewManager()
	if err != nil {
		return nil, err
	}
	instMgr.StartMonitor(context.Background())

	// 5. Wire them together (The Event Highway)
	ruleEng.Manager = instMgr
	instMgr.CostProvider = ruleEng

	// 10. Wire the OnRespond callback so VRon responses print to the CLI
	instMgr.OnRespond = func(response string) {
		PrintVRonResponse("msr", response)
		if err := ctxMgr.HistoryManager.Append("MSR", response); err != nil {
			utils.LogDebug("Failed to log MSR response to history: %v", err)
		}
	}

	// 11. Wire OnRetrieveLTM — called before each VRon executes to inject relevant memories
	instMgr.OnRetrieveLTM = func(query string) string {
		result, err := ctxMgr.SearchLTM(query, config.LTMMaxResults)
		if err != nil {
			utils.LogDebug("LTM retrieval failed: %v", err)
			return ""
		}
		return result
	}

	// 12. Wire OnSaveMemory — called when a VRon outputs "update_memory"
	instMgr.OnSaveMemory = func(content, epType string) {
		if err := ctxMgr.SaveEpisode(content, epType, config.LTMDefaultWeight); err != nil {
			utils.LogDebug("Failed to save episode: %v", err)
		}
	}

	return &AppCore{
		Manager:    instMgr,
		RuleEngine: ruleEng,
		Context:    ctxMgr,
	}, nil
}
