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
// Interface, Rule Engine, Scheduler, and Context Manager together.
type AppCore struct {
	Manager    *instanceManager.Manager
	RuleEngine *ruleEngine.ReflexDispatcher
	Scheduler  *ruleEngine.Scheduler
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
	ctxMgr, err := contextManager.NewContextManager()
	if err != nil {
		return nil, err
	}

	// 3. Initialize Rule Engine (Reflexes)
	ruleEng := ruleEngine.NewReflexDispatcher()

	// 4. Initialize Instance Manager (Engine Core)
	instMgr, err := instanceManager.NewManager()
	if err != nil {
		return nil, err
	}
	go instMgr.RunQueue(context.Background())
	instMgr.StartMonitor(context.Background(), ctxMgr.HistoryManager)

	// 5. Wire them together
	ruleEng.Manager = instMgr
	ruleEng.Context = ctxMgr
	instMgr.CostProvider = ruleEng

	// 6. Initialize Scheduler
	scheduler := ruleEngine.NewScheduler(instMgr, ctxMgr)

	// 7. On Startup: Load latest main consolidated episode (ep_*.json) into active in-memory context
	latestConsolidated, err := ctxMgr.GetLatestConsolidatedEpisode()
	if err == nil && latestConsolidated != "" {
		ctxMgr.HistoryManager.StartupContext = latestConsolidated
		utils.LogInfo("[AppCore] Restored latest consolidated episode into active memory context.")
	}

	// 10. Wire the OnRespond callback so VRon responses print to the CLI
	instMgr.OnRespond = func(response string) {
		PrintVRonResponse(config.OrganismName, response)
		if err := ctxMgr.HistoryManager.Append(config.OrganismName, response); err != nil {
			utils.LogDebug("Failed to log %s response to history: %v", config.OrganismName, err)
		}
	}

	// 11. Wire OnRetrieveLTM — called before each VRon executes to inject relevant memories
	instMgr.OnRetrieveLTM = func(query string) string {
		utils.LogInfo("[OnRetrieveLTM] Searching LTM for query: %q", query)
		result, err := ctxMgr.SearchLTM(query, config.LTMMaxResults)
		if err != nil {
			utils.LogInfo("[OnRetrieveLTM] Search failed: %v", err)
			return ""
		}
		utils.LogInfo("[OnRetrieveLTM] Search complete (%d chars retrieved)", len(result))
		return result
	}

	// Wire OnRetrieveLTMConsolidated — used by ContextSwap to exclude fact_ episodes
	instMgr.OnRetrieveLTMConsolidated = func(query string) string {
		result, err := ctxMgr.SearchLTMConsolidatedOnly(query, config.LTMMaxResults)
		if err != nil {
			utils.LogDebug("Consolidated LTM search failed: %v", err)
			return ""
		}
		return result
	}

	// 12. Wire OnSaveMemory — called when a VRon outputs single memories/facts
	instMgr.OnSaveMemory = func(content, epType string) {
		if err := ctxMgr.SaveEpisode(content, epType, config.LTMDefaultWeight); err != nil {
			utils.LogDebug("Failed to save episode: %v", err)
		}
	}

	// 13. Wire OnSaveSpecialEpisode — packages explicit facts into character-chunked fact_<timestamp> files
	instMgr.OnSaveSpecialEpisode = func(facts []string, epType string, mindState string) {
		if err := ctxMgr.SaveSpecialEpisode(facts, epType, mindState, config.LTMDefaultWeight, config.MaxResponseChars); err != nil {
			utils.LogDebug("Failed to save fact episode: %v", err)
		}
	}

	// 14. Wire OnSubconsciousTrigger - called during idle loops to spawn spontaneous thought
	instMgr.OnSubconsciousTrigger = func() {
		ruleEng.OnSubconsciousTrigger()
	}

	return &AppCore{
		Manager:    instMgr,
		RuleEngine: ruleEng,
		Scheduler:  scheduler,
		Context:    ctxMgr,
	}, nil
}
