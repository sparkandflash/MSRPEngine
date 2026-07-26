package instanceManager

import (
	"context"
	"fmt"
	"msrpe-vron-go/src/contextManager"
	"msrpe-vron-go/src/envconfig"
	"msrpe-vron-go/src/providers"
	"msrpe-vron-go/src/utils"
	"msrpe-vron-go/src/vron"
	"sync"
	"time"
)

// SleepMode defines the hibernation state of the engine.
type SleepMode int

const (
	ModeActive SleepMode = iota
	ModeUserIdle
	ModeHibernation
)

// CostProvider defines how the Manager requests energy ratios.
// Implemented by the Rule Engine.
type CostProvider interface {
	GetSpawnCostRatio() float64
	GetExecuteCostRatio() float64
	GetPassiveCostRatio() float64
	GetActiveRegenRatio() float64
	GetUserIdleRegenRatio() float64
	GetHibernationRegenRatio() float64
	GetHibernationSleepMult() float64
	GetCooldownPenaltyFactor() float64
}

// PendingVRon represents a VRon waiting in the queue.
type PendingVRon struct {
	ID          string
	ParentID    string
	Context     vron.VRonContext
	IsSuspended bool
	Instance    vron.VRon

	ThreadCost  int
	ThreadDepth int
	SpawnTime   time.Time
}

// Manager controls the lifecycle, rate limits, and energy pool of all VRons.
type Manager struct {
	mu           sync.Mutex
	GlobalEnergy        int
	MaxEnergy           int
	SerotoninLevel      int // Bipolar Mind Score (-100 to +100)
	BaseTick            time.Duration
	TickCooldown        time.Duration
	SleepMode    SleepMode
	Provider     providers.InferenceProvider // The live LLM backend
	CostProvider CostProvider                // Injected by Rule Engine
	
	FatigueThresholdPct float64              // Injected by AppCore from Env
	BaseLifetime        time.Duration
	UserIdleTimeout     time.Duration
	HibernationTimeout  time.Duration

	// Callbacks wired by AppCore
	OnRespond     func(response string)          // Called when a VRon returns "respond"
	OnRetrieveLTM func(query string) string      // Called before VRon executes to populate LTM
	OnSaveMemory  func(content, epType string)   // Called when a VRon returns "update_memory"
	OnSubconsciousTrigger func()                 // Called when the engine is idle and has energy to spawn spontaneous thoughts

	queue          []*PendingVRon
	priorityQueue  []*PendingVRon
	suspended      map[string]*PendingVRon
	activeVRons    int
	lastEnergyBurn int
	lastActivity   time.Time
}

// GetEnergy returns the current global energy level in a thread-safe way.
// Use this instead of reading Manager.GlobalEnergy directly.
func (m *Manager) GetEnergy() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.GlobalEnergy
}

// GetConsumptionRate returns the last observed energy burn rate in a thread-safe way.
func (m *Manager) GetConsumptionRate() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastEnergyBurn
}

// NewManager creates a new InstanceManager with rules-driven constraints.
// It initializes its own InferenceProvider and extracts limits from envconfig.
func NewManager() (*Manager, error) {
	config := envconfig.Load()
	
	provider, err := providers.NewInferenceProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize InferenceProvider: %v", err)
	}

	return &Manager{
		GlobalEnergy:        config.MaxGlobalEnergy,
		MaxEnergy:           config.MaxGlobalEnergy,
		SerotoninLevel:      0, // Start neutral
		BaseTick:            config.BaseTickRateMs,
		TickCooldown:        config.BaseTickRateMs,
		SleepMode:           ModeActive,
		Provider:            provider,
		FatigueThresholdPct: config.FatigueThresholdPct,
		BaseLifetime:        config.BaseLifetime,
		UserIdleTimeout:     config.UserIdleTimeout,
		HibernationTimeout:  config.HibernationTimeout,
		queue:               make([]*PendingVRon, 0),
		priorityQueue:       make([]*PendingVRon, 0),
		suspended:           make(map[string]*PendingVRon),
		lastActivity:        time.Now(),
	}, nil
}

// RecordUserActivity bumps the activity timer, keeping the engine in ModeActive.
func (m *Manager) RecordUserActivity() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastActivity = time.Now()
	m.SleepMode = ModeActive
}

// StartMonitor begins tracking idle state in the background.
func (m *Manager) StartMonitor(ctx context.Context, historyMgr *contextManager.InterfaceHistoryManager) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.mu.Lock()
				idleDuration := time.Since(m.lastActivity)
				
				isExhausted := m.GlobalEnergy <= int(float64(m.MaxEnergy)*m.FatigueThresholdPct)

				if idleDuration >= m.HibernationTimeout || isExhausted {
					if m.SleepMode != ModeHibernation {
						utils.LogInfo("[InstanceManager] Engine shifting to Hibernation (Idle OR Exhausted)")
						m.SleepMode = ModeHibernation
						if historyMgr != nil {
							_ = historyMgr.Append("System", "Biological shift: Hibernation Mode activated due to inactivity or energy exhaustion. Metabolic rate is reduced.")
						}
					}
				} else if idleDuration >= m.UserIdleTimeout {
					if m.SleepMode != ModeUserIdle && m.SleepMode != ModeHibernation {
						utils.LogInfo("[InstanceManager] Engine shifting to UserIdle")
						m.SleepMode = ModeUserIdle
						if historyMgr != nil {
							_ = historyMgr.Append("System", "Biological shift: User Idle Mode activated. The environment is quiet.")
						}
					}
					// Subconscious thought generation during UserIdle (if we have energy)
					if m.SleepMode == ModeUserIdle && m.OnSubconsciousTrigger != nil && !isExhausted {
						m.OnSubconsciousTrigger()
					}
				}
				m.mu.Unlock()
			}
		}
	}()
}

// adjustSerotonin safely modifies the SerotoninLevel, capping between -100 and +100.
func (m *Manager) adjustSerotonin(delta int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SerotoninLevel += delta
	if m.SerotoninLevel > 100 {
		m.SerotoninLevel = 100
	} else if m.SerotoninLevel < -100 {
		m.SerotoninLevel = -100
	}
}

// Spawn enqueues a new VRon for execution. It applies biological backpressure.
func (m *Manager) Spawn(parentID string, vronCtx vron.VRonContext, instance vron.VRon) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Biological backpressure: Refuse background spawns if energy is critically low
	criticalThreshold := int(float64(m.MaxEnergy) * m.FatigueThresholdPct)
	if m.GlobalEnergy <= criticalThreshold && parentID != "" {
		return fmt.Errorf(vron.SysMsgFatigue)
	}

	spawnCost := int(float64(m.MaxEnergy) * 0.02) // Fallback
	if m.CostProvider != nil {
		spawnCost = int(float64(m.MaxEnergy) * m.CostProvider.GetSpawnCostRatio())
	}

	// Cost scaling: Deduct energy for the spawn
	m.GlobalEnergy -= spawnCost
	if m.GlobalEnergy < 0 {
		m.GlobalEnergy = 0
	}

	// Dynamic Cooldown logic
	// Formula: BaseTick + (DeficitPct / 0.10 * PenaltyFactor * BaseTick)
	penaltyFactor := 0.5
	if m.CostProvider != nil {
		penaltyFactor = m.CostProvider.GetCooldownPenaltyFactor()
	}
	
	energyDeficitPct := float64(m.MaxEnergy - m.GlobalEnergy) / float64(m.MaxEnergy)
	deficitIncrements := energyDeficitPct / 0.10 // How many 10% blocks are we missing?
	penaltyMs := deficitIncrements * penaltyFactor * float64(m.BaseTick.Milliseconds())
	
	m.TickCooldown = m.BaseTick + time.Duration(penaltyMs)*time.Millisecond

	// Calculate Thread Depth and Cost
	childDepth := 1
	childCost := spawnCost

	if parentID != "" {
		if parent, exists := m.suspended[parentID]; exists {
			childDepth = parent.ThreadDepth + 1
			childCost = parent.ThreadCost + spawnCost
		} else {
			// BUG FIX #2: Parent not found in suspended map. Thread cost chain is broken.
			// This can happen if the parent completed before the child was spawned.
			utils.LogInfo("[InstanceManager] WARNING: ParentID %s not found in suspended map. Thread cost chain broken. Child will start at Depth=1.", parentID)
		}
	}

	v := &PendingVRon{
		ID:          fmt.Sprintf("vron-%d", time.Now().UnixNano()),
		ParentID:    parentID,
		Context:     vronCtx,
		Instance:    instance,
		ThreadCost:  childCost,
		ThreadDepth: childDepth,
		SpawnTime:   time.Now(),
	}

	utils.LogDebug("VRon Created | ID: %s | Depth: %d | ThreadCost: %d | Energy: %d | New Cooldown: %v", v.ID, v.ThreadDepth, v.ThreadCost, m.GlobalEnergy, m.TickCooldown)

	m.queue = append(m.queue, v)
	return nil
}

// RunQueue starts the background processing loop.
func (m *Manager) RunQueue(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// BUG FIX #4: Snapshot TickCooldown under the lock to prevent a data race.
			// TickCooldown is written inside Spawn() under m.mu, so we must read
			// it under the same lock before sleeping.
			m.mu.Lock()
			cooldown := m.TickCooldown
			sleepMode := m.SleepMode
			sleepMult := 2.5
			if m.CostProvider != nil {
				sleepMult = m.CostProvider.GetHibernationSleepMult()
			}
			m.mu.Unlock()
			
			// Dynamic Sleep for Hibernation (outside lock)
			if sleepMode == ModeHibernation {
				time.Sleep(time.Duration(float64(m.BaseTick.Milliseconds()) * sleepMult) * time.Millisecond)
			} else {
				time.Sleep(cooldown)
			}

			m.mu.Lock()
			
			// Passive energy drain & Serotonin Homeostasis (decay towards 0)
			alive := len(m.priorityQueue) + m.activeVRons
			if alive > 0 {
				drain := 0
				if m.CostProvider != nil {
					drain = int(float64(m.MaxEnergy) * m.CostProvider.GetPassiveCostRatio() * float64(alive))
				} else {
					drain = alive // fallback
				}
				m.GlobalEnergy -= drain
				if m.GlobalEnergy < 0 {
					m.GlobalEnergy = 0
				}
				utils.LogDebug("[Energy] Passive drain for %d alive VRons (-%d) | Energy: %d", alive, drain, m.GlobalEnergy)
			}
			
			// Serotonin Homeostasis: pull back towards 0 over time
			if m.SerotoninLevel > 0 {
				m.SerotoninLevel--
			} else if m.SerotoninLevel < 0 {
				m.SerotoninLevel++
			}

			// Sweep for Decayed VRons (Biological Decay)
			if m.MaxEnergy > 0 {
				energyPct := float64(m.GlobalEnergy) / float64(m.MaxEnergy)
				// Never let energyPct drop below a tiny minimum to avoid instantaneous decay (div by near-zero)
				if energyPct < 0.05 {
					energyPct = 0.05
				}
				dynamicLifetime := time.Duration(float64(m.BaseLifetime.Nanoseconds()) * energyPct)
				
				// Helper to filter decayed VRons
				filterDecayed := func(q []*PendingVRon, name string) []*PendingVRon {
					active := make([]*PendingVRon, 0, len(q))
					for _, v := range q {
						if time.Since(v.SpawnTime) > dynamicLifetime {
							utils.LogInfo("[InstanceManager] VRon %s decayed after %v (Energy: %.0f%%)", v.ID, time.Since(v.SpawnTime), energyPct*100)
						} else {
							active = append(active, v)
						}
					}
					return active
				}

				m.queue = filterDecayed(m.queue, "queue")
				m.priorityQueue = filterDecayed(m.priorityQueue, "priorityQueue")

				// Filter suspended map
				for id, v := range m.suspended {
					if time.Since(v.SpawnTime) > dynamicLifetime {
						utils.LogInfo("[InstanceManager] Suspended VRon %s decayed after %v (Energy: %.0f%%)", v.ID, time.Since(v.SpawnTime), energyPct*100)
						delete(m.suspended, id)
					}
				}
			}

			if len(m.queue) == 0 && len(m.priorityQueue) == 0 {
				// --- Energy Recovery (ported from V2 msrpEngine-go) ---
				// Mirrors V2's two-tier regen: more recovery in deeper sleep modes.
				regenActive := int(float64(m.MaxEnergy) * 0.01)
				regenUserIdle := int(float64(m.MaxEnergy) * 0.02)
				regenHibernation := int(float64(m.MaxEnergy) * 0.05)
				if m.CostProvider != nil {
					regenActive = int(float64(m.MaxEnergy) * m.CostProvider.GetActiveRegenRatio())
					regenUserIdle = int(float64(m.MaxEnergy) * m.CostProvider.GetUserIdleRegenRatio())
					regenHibernation = int(float64(m.MaxEnergy) * m.CostProvider.GetHibernationRegenRatio())
				}

				switch m.SleepMode {
				case ModeHibernation:
					m.GlobalEnergy += regenHibernation
				case ModeUserIdle:
					m.GlobalEnergy += regenUserIdle
				case ModeActive:
					m.GlobalEnergy += regenActive
				}
				if m.GlobalEnergy > m.MaxEnergy {
					m.GlobalEnergy = m.MaxEnergy
				}
				utils.LogDebug("[Energy] Idle recovery | Energy: %d/%d", m.GlobalEnergy, m.MaxEnergy)
				m.mu.Unlock()
				continue
			}

			// Pop the first pending VRon (Priority Queue first)
			var nextVRon *PendingVRon
			if len(m.priorityQueue) > 0 {
				nextVRon = m.priorityQueue[0]
				m.priorityQueue = m.priorityQueue[1:]
			} else {
				nextVRon = m.queue[0]
				m.queue = m.queue[1:]
			}
			
			m.activeVRons++

			// Inject dynamic metrics right before execution
			nextVRon.Context.EnergyLevel = m.GlobalEnergy
			nextVRon.Context.MaxEnergy = m.MaxEnergy
			nextVRon.Context.ConsumptionRate = m.lastEnergyBurn
			nextVRon.Context.ActiveVRons = m.activeVRons
			nextVRon.Context.ThreadCost = nextVRon.ThreadCost
			nextVRon.Context.ThreadDepth = nextVRon.ThreadDepth
			m.mu.Unlock()

			// Execute the VRon (this would block, representing LLM time)
			// For async pausing: If the VRon returns a "spawn_child" action,
			// we suspend it, put it in the suspended map, and spawn the child.
			// When the child finishes, we pull it from suspended and re-queue it.
			utils.LogDebug("Executing VRon %s | Depth: %d | Cost: %d | Active: %d | Rate: %d",
				nextVRon.ID, nextVRon.ThreadDepth, nextVRon.ThreadCost,
				nextVRon.Context.ActiveVRons, nextVRon.Context.ConsumptionRate)

			// Populate LTM via callback before calling the LLM
			if m.OnRetrieveLTM != nil {
				ltm := m.OnRetrieveLTM(nextVRon.Context.STM)
				nextVRon.Context.LTM = ltm
				utils.LogDebug("LTM Injected | VRon %s | %d chars", nextVRon.ID, len(ltm))
			}

			// BUG FIX #3: Use defer to guarantee activeVRons is ALWAYS decremented,
			// even if the LLM call panics or returns a fatal error.
			func() {
				defer func() {
					m.mu.Lock()
					energyBefore := m.GlobalEnergy
					executeCost := int(float64(m.MaxEnergy) * 0.01)
					if m.CostProvider != nil {
						executeCost = int(float64(m.MaxEnergy) * m.CostProvider.GetExecuteCostRatio())
					}
					m.GlobalEnergy -= executeCost
					if m.GlobalEnergy < 0 {
						m.GlobalEnergy = 0
					}
					m.lastEnergyBurn = energyBefore - m.GlobalEnergy
					m.activeVRons--
					m.mu.Unlock()
				}()

				// Inject dynamic metrics right before execution
				nextVRon.Context.EnergyLevel = m.GlobalEnergy
				nextVRon.Context.SerotoninLevel = m.SerotoninLevel

				// Detailed VRon logging (In)
				utils.LogDebug("[LLM-IN] VRon %s | Action: Executing | STM Chars: %d | LTM Chars: %d | Depth: %d | SE: %d", nextVRon.ID, len(nextVRon.Context.STM), len(nextVRon.Context.LTM), nextVRon.ThreadDepth, nextVRon.Context.SerotoninLevel)

				// Inject LTM via callback right before LLM call
				if m.OnRetrieveLTM != nil {
					nextVRon.Context.LTM = m.OnRetrieveLTM(nextVRon.Context.STM)
				}

				output, err := m.Provider.GenerateStructured(ctx, vron.GetMasterVRonPrompt(), nextVRon.Context)
				if err != nil {
					utils.LogDebug("[LLM-FAIL] VRon %s execution failed: %v", nextVRon.ID, err)
					m.adjustSerotonin(-10) // Negative reinforcement for failing to execute
					return
				}

				// Detailed VRon logging (Out)
				utils.LogDebug("[LLM-OUT] VRon %s | Action: '%s' | Confidence: %d | Response Chars: %d", nextVRon.ID, output.Action, output.Confidence, len(output.Query))

				utils.LogDebug("VRon %s → Action: %s | Goal: %s", nextVRon.ID, output.Action, output.Goal)

				// Dispatch based on VRon decision
				switch output.Action {
				case "respond":
					m.adjustSerotonin(10) // Positive reinforcement for successful interaction
					// Route response back to interface
					if m.OnRespond != nil {
						m.OnRespond(output.Query)
					}
					// Parent resume: pass child result and this VRon's thread cost up the chain
					m.resumeParent(nextVRon.ParentID, output.Query, nextVRon.ThreadCost)

				case "spawn_child":
					m.adjustSerotonin(5) // Reaching/Instinctual drive 
					// Suspend parent, enqueue child
					m.mu.Lock()
					m.suspended[nextVRon.ID] = nextVRon
					m.mu.Unlock()

					childCtx := vron.VRonContext{
						PassedContext: output.Query,
						Goal:          output.Goal,
						STM:          nextVRon.Context.STM,
					}
					err := m.Spawn(nextVRon.ID, childCtx, nil)
					if err != nil {
						utils.LogInfo("[InstanceManager] Failed to spawn child from VRon %s: %v", nextVRon.ID, err)
						// Unsuspend parent if child failed to spawn
						m.mu.Lock()
						delete(m.suspended, nextVRon.ID)
						m.mu.Unlock()
						m.resumeParent(nextVRon.ParentID, "ERROR: child spawn failed", nextVRon.ThreadCost)
					}

				case "update_memory":
					threshold := 80
					if nextVRon.Context.SerotoninLevel < 0 {
						threshold = 95
					}

					if output.Confidence < threshold {
						utils.LogDebug("[InstanceManager] VRon %s attempted update_memory with low confidence (%d < %d). Blocked.", nextVRon.ID, output.Confidence, threshold)
						m.adjustSerotonin(-10) // Severe penalty for attempting to hallucinate memory
						m.resumeParent(nextVRon.ParentID, fmt.Sprintf("FAIL: Confidence too low to commit fact. Required: %d", threshold), nextVRon.ThreadCost)
						return
					}

					m.adjustSerotonin(15) // High reward for successfully extracting and committing knowledge
					if m.OnSaveMemory != nil {
						m.OnSaveMemory(output.Query, "fact")
					}
					m.resumeParent(nextVRon.ParentID, "Memory successfully updated.", nextVRon.ThreadCost)

				case "test_result":
					if output.Query == "FAIL" || output.Query == "UNKNOWN" {
						m.adjustSerotonin(-10) // Agitation/Frustration
					} else {
						m.adjustSerotonin(10) // Satisfaction
					}
					m.resumeParent(nextVRon.ParentID, output.Query, nextVRon.ThreadCost)
					
					// Also route to OnSaveMemory if it's a test_result
					if m.OnSaveMemory != nil {
						m.OnSaveMemory(output.Query, "test_result")
					}
				}
			}()
		}
	}
}

// resumeParent wakes a suspended parent VRon by injecting the child's result
// as PassedContext and re-enqueuing it. childCost is the terminal ThreadCost of
// the child, which is accumulated onto the parent so the LLM sees the true chain cost.
func (m *Manager) resumeParent(parentID string, childResult string, childCost int) {
	if parentID == "" {
		return
	}
	m.mu.Lock()
	parent, exists := m.suspended[parentID]
	if !exists {
		m.mu.Unlock()
		return
	}
	delete(m.suspended, parentID)

	// Inject child result into parent's PassedContext so it has the answer
	parent.Context.PassedContext = childResult
	// Accumulate child's total cost into parent's ThreadCost so the LLM
	// always sees the real cumulative cost of the entire chain.
	parent.ThreadCost += childCost
	m.priorityQueue = append(m.priorityQueue, parent)
	m.mu.Unlock()

	utils.LogDebug("[InstanceManager] Parent %s resumed | child result: %d chars | accumulated thread cost: %d", parentID, len(childResult), parent.ThreadCost)
}
