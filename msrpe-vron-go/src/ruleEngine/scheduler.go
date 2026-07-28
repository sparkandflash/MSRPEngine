package ruleEngine

import (
	"context"
	"fmt"
	"time"

	"msrpe-vron-go/src/contextManager"
	"msrpe-vron-go/src/envconfig"
	"msrpe-vron-go/src/instanceManager"
	"msrpe-vron-go/src/utils"
	"msrpe-vron-go/src/vron"
)

// Scheduler coordinates deterministic method assignment to VRons.
type Scheduler struct {
	Manager *instanceManager.Manager
	Context *contextManager.ContextManager
}

// NewScheduler creates a new deterministic scheduler instance.
func NewScheduler(mgr *instanceManager.Manager, ctx *contextManager.ContextManager) *Scheduler {
	return &Scheduler{
		Manager: mgr,
		Context: ctx,
	}
}

// StartBackgroundSchedulers begins periodic ticks (e.g. ContextSwap every 5 minutes).
func (s *Scheduler) StartBackgroundSchedulers(ctx context.Context) {
	config := envconfig.Load()
	swapInterval := config.ContextSwapInterval
	if swapInterval <= 0 {
		swapInterval = 300 * time.Second
	}

	go func() {
		ticker := time.NewTicker(swapInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.TriggerContextSwap()
			}
		}
	}()
}

// TriggerUserMessage dispatches a MethodRespond VRon when a user message arrives.
func (s *Scheduler) TriggerUserMessage(message string) error {
	utils.LogInfo("[Scheduler] Triggering User Message: %s", message)
	s.Manager.RecordUserActivity()

	stmContext := ""
	if s.Context != nil && s.Context.HistoryManager != nil {
		stmContext = s.Context.HistoryManager.ReadRecentContext(4000)
	}
	if stmContext == "" {
		stmContext = fmt.Sprintf("User says: %s", message)
	}

	ctx := vron.VRonContext{
		STM:             stmContext,
		Method:          vron.MethodRespond,
		Goal:            string(vron.MethodRespond),
		EnergyLevel:     s.Manager.GetEnergy(),
		ConsumptionRate: s.Manager.GetConsumptionRate(),
	}

	err := s.Manager.Spawn("", ctx, nil)
	utils.LogInfo("[Scheduler] Spawn completed with err: %v", err)
	return err
}

// TriggerSubconscious dispatches a MethodReact VRon during idle state.
func (s *Scheduler) TriggerSubconscious() error {
	stmContext := ""
	if s.Context != nil && s.Context.HistoryManager != nil {
		stmContext = s.Context.HistoryManager.ReadRecentContext(4000)
	}
	stmContext += "\n[System]: You are idle. The environment is quiet."

	ctx := vron.VRonContext{
		STM:             stmContext,
		Method:          vron.MethodReact,
		Goal:            string(vron.MethodReact),
		EnergyLevel:     s.Manager.GetEnergy(),
		ConsumptionRate: s.Manager.GetConsumptionRate(),
	}

	return s.Manager.Spawn("", ctx, nil)
}

// TriggerConsolidation dispatches a MethodConsolidate VRon.
func (s *Scheduler) TriggerConsolidation() error {
	stmContext := ""
	if s.Context != nil && s.Context.HistoryManager != nil {
		stmContext = s.Context.HistoryManager.ReadRecentContext(8000)
	}

	if s.Context != nil {
		activeFacts := s.Context.GetAllStoredFactsFormatted()
		if activeFacts != "" {
			stmContext = activeFacts + "\n\n" + stmContext
		}
	}

	ctx := vron.VRonContext{
		STM:             stmContext,
		Method:          vron.MethodConsolidate,
		Goal:            string(vron.MethodConsolidate),
		EnergyLevel:     s.Manager.GetEnergy(),
		ConsumptionRate: s.Manager.GetConsumptionRate(),
	}

	return s.Manager.Spawn("", ctx, nil)
}

// TriggerContextSwap dispatches a MethodContextSwap VRon.
func (s *Scheduler) TriggerContextSwap() error {
	utils.LogInfo("[Scheduler] Triggering scheduled ContextSwap")
	stmContext := ""
	if s.Context != nil && s.Context.HistoryManager != nil {
		stmContext = s.Context.HistoryManager.ReadRecentContext(4000)
	}

	ctx := vron.VRonContext{
		STM:             stmContext,
		Method:          vron.MethodContextSwap,
		Goal:            string(vron.MethodContextSwap),
		EnergyLevel:     s.Manager.GetEnergy(),
		ConsumptionRate: s.Manager.GetConsumptionRate(),
	}

	return s.Manager.Spawn("", ctx, nil)
}
