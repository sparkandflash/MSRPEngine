package instanceManager

import (
	"context"
	"fmt"
	"msrpe-vron-go/src/contextManager"
	"msrpe-vron-go/src/envconfig"
	"msrpe-vron-go/src/providers"
	"msrpe-vron-go/src/utils"
	"msrpe-vron-go/src/vron"
	"strings"
	"sync"
	"sync/atomic"
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

// PendingVRon represents a VRon cell in the engine queue or lifecycle.
type PendingVRon struct {
	ID           string
	ParentID     string
	WaitingForID string
	Method       vron.Method
	Status       vron.VRonStatus
	Context      vron.VRonContext
	Instance     vron.VRon

	ThreadCost  int
	ThreadDepth int
	SpawnTime   time.Time
}

// Manager controls the lifecycle, rate limits, and energy pool of all VRons.
type Manager struct {
	mu           sync.Mutex
	vronCounter  atomic.Int64 // Monotonically increasing counter for unique VRon IDs
	GlobalEnergy int
	MaxEnergy    int
	SerotoninLevel int // Biological Mind Score (-100 to +100)
	BaseTick     time.Duration
	TickCooldown time.Duration
	SleepMode    SleepMode
	Provider     providers.InferenceProvider // The live LLM backend
	CostProvider CostProvider                // Injected by Rule Engine

	FatigueThresholdPct float64
	BaseLifetime        time.Duration
	UserIdleTimeout     time.Duration
	HibernationTimeout  time.Duration
	MaxContextChars     int

	// Callbacks wired by AppCore
	OnRespond             func(response string)
	OnRetrieveLTM         func(query string) string
	OnSaveMemory          func(content, epType string)
	OnSubconsciousTrigger func()

	vronPool       chan *PendingVRon
	queue          []*PendingVRon
	priorityQueue  []*PendingVRon
	suspended      map[string]*PendingVRon
	activeVRons    int
	lastEnergyBurn int
	lastActivity   time.Time
}

// GetEnergy returns the current global energy level in a thread-safe way.
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
func NewManager() (*Manager, error) {
	config := envconfig.Load()

	provider, err := providers.NewInferenceProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize InferenceProvider: %v", err)
	}

	m := &Manager{
		GlobalEnergy:        config.MaxGlobalEnergy,
		MaxEnergy:           config.MaxGlobalEnergy,
		SerotoninLevel:      0,
		BaseTick:            config.BaseTickRateMs,
		TickCooldown:        config.BaseTickRateMs,
		SleepMode:           ModeActive,
		Provider:            provider,
		FatigueThresholdPct: config.FatigueThresholdPct,
		BaseLifetime:        config.BaseLifetime,
		UserIdleTimeout:     config.UserIdleTimeout,
		HibernationTimeout:  config.HibernationTimeout,
		MaxContextChars:     config.MaxContextChars,
		queue:               make([]*PendingVRon, 0),
		priorityQueue:       make([]*PendingVRon, 0),
		suspended:           make(map[string]*PendingVRon),
		vronPool:            make(chan *PendingVRon, config.VRonPoolMaxCapacity),
		lastActivity:        time.Now(),
	}

	// Pre-fill the VRon pool
	for i := 0; i < config.VRonPoolMaxCapacity; i++ {
		m.vronPool <- m.createEmptyVRon()
	}

	return m, nil
}

// createEmptyVRon generates a new uninitialized biological cell with a guaranteed unique ID.
func (m *Manager) createEmptyVRon() *PendingVRon {
	id := m.vronCounter.Add(1)
	return &PendingVRon{
		ID:     fmt.Sprintf("vron-%d", id),
		Status: vron.StatusIdle,
	}
}

// StartPoolRegen handles batch cell pool refilling every refillRate (e.g. 60s).
func (m *Manager) StartPoolRegen(ctx context.Context, refillRate time.Duration) {
	go func() {
		ticker := time.NewTicker(refillRate)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.mu.Lock()
				// Drain any empty/unclaimed cells from the channel
			drainLoop:
				for {
					select {
					case <-m.vronPool:
					default:
						break drainLoop
					}
				}
				// Refill pool back to full max capacity
				maxCap := cap(m.vronPool)
				if maxCap == 0 {
					maxCap = 12
				}
				for i := 0; i < maxCap; i++ {
					select {
					case m.vronPool <- m.createEmptyVRon():
					default:
					}
				}
				utils.LogInfo("[InstanceManager] VRon cell pool batch refilled to capacity (%d cells)", maxCap)
				m.mu.Unlock()
			}
		}
	}()
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
				currentMode := m.SleepMode

				var triggerSubconscious bool

				if idleDuration >= m.HibernationTimeout || isExhausted {
					if currentMode != ModeHibernation {
						utils.LogInfo("[InstanceManager] Engine shifting to Hibernation (Idle OR Exhausted)")
						m.SleepMode = ModeHibernation
						if historyMgr != nil {
							m.mu.Unlock()
							_ = historyMgr.Append("System", "Biological shift: Hibernation Mode activated due to inactivity or energy exhaustion.")
							continue
						}
					}
				} else if idleDuration >= m.UserIdleTimeout {
					if currentMode != ModeUserIdle && currentMode != ModeHibernation {
						utils.LogInfo("[InstanceManager] Engine shifting to UserIdle")
						m.SleepMode = ModeUserIdle
						if historyMgr != nil {
							m.mu.Unlock()
							_ = historyMgr.Append("System", "Biological shift: User Idle Mode activated. The environment is quiet.")
							continue
						}
					}
					if m.SleepMode == ModeUserIdle && m.OnSubconsciousTrigger != nil && !isExhausted {
						triggerSubconscious = true
					}
				}
				m.mu.Unlock()

				if triggerSubconscious {
					m.OnSubconsciousTrigger()
				}
			}
		}
	}()
}

// adjustSerotonin safely modifies SerotoninLevel, capping between -100 and +100.
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

// Spawn enqueues a new VRon for execution with an assigned Method.
func (m *Manager) Spawn(parentID string, vronCtx vron.VRonContext, instance vron.VRon) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	criticalThreshold := int(float64(m.MaxEnergy) * m.FatigueThresholdPct)
	if m.GlobalEnergy <= criticalThreshold && parentID != "" {
		return fmt.Errorf(vron.SysMsgFatigue)
	}

	var v *PendingVRon
	select {
	case v = <-m.vronPool:
	default:
		utils.LogInfo("[InstanceManager] VRon pool exhausted. Waiting for batch regeneration...")
		return fmt.Errorf("VRon pool exhausted, rate limit hit")
	}

	spawnCost := int(float64(m.MaxEnergy) * 0.02)
	if m.CostProvider != nil {
		spawnCost = int(float64(m.MaxEnergy) * m.CostProvider.GetSpawnCostRatio())
	}

	m.GlobalEnergy -= spawnCost
	if m.GlobalEnergy < 0 {
		m.GlobalEnergy = 0
	}

	penaltyFactor := 0.5
	if m.CostProvider != nil {
		penaltyFactor = m.CostProvider.GetCooldownPenaltyFactor()
	}
	energyDeficitPct := float64(m.MaxEnergy-m.GlobalEnergy) / float64(m.MaxEnergy)
	deficitIncrements := energyDeficitPct / 0.10
	penaltyMs := deficitIncrements * penaltyFactor * float64(m.BaseTick.Milliseconds())
	m.TickCooldown = m.BaseTick + time.Duration(penaltyMs)*time.Millisecond

	childDepth := 1
	childCost := spawnCost

	if parentID != "" {
		if parent, exists := m.suspended[parentID]; exists {
			childDepth = parent.ThreadDepth + 1
			childCost = parent.ThreadCost + spawnCost
		}
	}

	if vronCtx.Method == "" {
		vronCtx.Method = vron.MethodRespond
	}
	vronCtx.Goal = string(vronCtx.Method)

	v.ParentID = parentID
	v.Method = vronCtx.Method
	v.Status = vron.StatusNew
	v.Context = vronCtx
	v.Instance = instance
	v.ThreadCost = childCost
	v.ThreadDepth = childDepth
	v.SpawnTime = time.Now()

	utils.LogDebug("VRon Created | ID: %s | Method: %s | Depth: %d | Cost: %d | Energy: %d", v.ID, v.Method, v.ThreadDepth, v.ThreadCost, m.GlobalEnergy)

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
			m.mu.Lock()
			cooldown := m.TickCooldown
			sleepMode := m.SleepMode
			sleepMult := 2.5
			if m.CostProvider != nil {
				sleepMult = m.CostProvider.GetHibernationSleepMult()
			}
			m.mu.Unlock()

			if sleepMode == ModeHibernation {
				time.Sleep(time.Duration(float64(m.BaseTick.Milliseconds())*sleepMult) * time.Millisecond)
			} else {
				time.Sleep(cooldown)
			}

			m.mu.Lock()

			alive := len(m.priorityQueue) + m.activeVRons
			if alive > 0 {
				drain := 0
				if m.CostProvider != nil {
					drain = int(float64(m.MaxEnergy) * m.CostProvider.GetPassiveCostRatio() * float64(alive))
				} else {
					drain = alive
				}
				m.GlobalEnergy -= drain
				if m.GlobalEnergy < 0 {
					m.GlobalEnergy = 0
				}
			}

			if m.SerotoninLevel > 0 {
				m.SerotoninLevel--
			} else if m.SerotoninLevel < 0 {
				m.SerotoninLevel++
			}

			if len(m.queue) == 0 && len(m.priorityQueue) == 0 {
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
				m.mu.Unlock()
				continue
			}

			var nextVRon *PendingVRon
			if len(m.priorityQueue) > 0 {
				nextVRon = m.priorityQueue[0]
				m.priorityQueue = m.priorityQueue[1:]
			} else {
				nextVRon = m.queue[0]
				m.queue = m.queue[1:]
			}

			nextVRon.Status = vron.StatusActive
			m.activeVRons++

			maxChars := m.MaxContextChars
			if maxChars <= 0 {
				maxChars = 10000
			}
			if len(nextVRon.Context.STM) > maxChars {
				nextVRon.Context.STM = nextVRon.Context.STM[len(nextVRon.Context.STM)-maxChars:]
			}

			nextVRon.Context.EnergyLevel = m.GlobalEnergy
			nextVRon.Context.MaxEnergy = m.MaxEnergy
			nextVRon.Context.ConsumptionRate = m.lastEnergyBurn
			nextVRon.Context.ActiveVRons = m.activeVRons
			nextVRon.Context.ThreadCost = nextVRon.ThreadCost
			nextVRon.Context.ThreadDepth = nextVRon.ThreadDepth
			m.mu.Unlock()

			utils.LogDebug("Executing VRon %s | Method: %s | Depth: %d | Cost: %d", nextVRon.ID, nextVRon.Method, nextVRon.ThreadDepth, nextVRon.ThreadCost)

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

				nextVRon.Context.EnergyLevel = m.GlobalEnergy
				nextVRon.Context.SerotoninLevel = m.SerotoninLevel

				prompt := vron.GetMethodPrompt(nextVRon.Method)

				switch nextVRon.Method {
				case vron.MethodRespond:
					if m.OnRetrieveLTM != nil && nextVRon.Context.LTM == "" && nextVRon.Context.PassedContext == "" {
						ltmQuery := string(nextVRon.Method)
						if stm := nextVRon.Context.STM; stm != "" {
							lines := strings.Split(strings.TrimSpace(stm), "\n")
							for i := len(lines) - 1; i >= 0; i-- {
								if trimmed := strings.TrimSpace(lines[i]); trimmed != "" {
									ltmQuery = ltmQuery + " " + trimmed
									break
								}
							}
						}
						nextVRon.Context.LTM = m.OnRetrieveLTM(ltmQuery)
					}

					output, err := m.Provider.GenerateStructured(ctx, prompt, nextVRon.Context)
					if err != nil {
						utils.LogDebug("[VRon-FAIL] %s execution error: %v", nextVRon.ID, err)
						m.adjustSerotonin(-10)
						nextVRon.Status = vron.StatusTerminated
						return
					}

					// Method chaining check: If LLM indicates memory or test is needed, spawn child VRon
					if (output.NeedMemory && output.MemoryQuery != "") || (output.MemoryQuery != "" && nextVRon.Context.PassedContext == "") {
						m.mu.Lock()
						m.suspended[nextVRon.ID] = nextVRon
						nextVRon.Status = vron.StatusWaiting
						m.mu.Unlock()

						childCtx := vron.VRonContext{
							PassedContext: output.MemoryQuery,
							Method:        vron.MethodQueryMemory,
							Goal:          string(vron.MethodQueryMemory),
							STM:           nextVRon.Context.STM,
						}
						err := m.Spawn(nextVRon.ID, childCtx, nil)
						if err == nil {
							utils.LogInfo("[InstanceManager] VRon %s suspended waiting for QueryMemory child", nextVRon.ID)
							return
						}
						// If spawn fails, unsuspend and proceed with response
						m.mu.Lock()
						delete(m.suspended, nextVRon.ID)
						nextVRon.Status = vron.StatusActive
						m.mu.Unlock()
					} else if output.NeedTest && output.TestQuery != "" {
						m.mu.Lock()
						m.suspended[nextVRon.ID] = nextVRon
						nextVRon.Status = vron.StatusWaiting
						m.mu.Unlock()

						childCtx := vron.VRonContext{
							PassedContext: output.TestQuery,
							Method:        vron.MethodTest,
							Goal:          string(vron.MethodTest),
							STM:           nextVRon.Context.STM,
						}
						err := m.Spawn(nextVRon.ID, childCtx, nil)
						if err == nil {
							utils.LogInfo("[InstanceManager] VRon %s suspended waiting for Test child", nextVRon.ID)
							return
						}
						m.mu.Lock()
						delete(m.suspended, nextVRon.ID)
						nextVRon.Status = vron.StatusActive
						m.mu.Unlock()
					}

					respText := output.Response
					if respText == "" {
						respText = output.Query
					}

					if respText != "" && m.OnRespond != nil {
						m.OnRespond(respText)
						m.adjustSerotonin(10)
					}

					nextVRon.Status = vron.StatusTerminated
					m.resumeParent(nextVRon.ParentID, respText, nextVRon.ThreadCost)

				case vron.MethodConsolidate:
					output, err := m.Provider.GenerateStructured(ctx, prompt, nextVRon.Context)
					if err != nil {
						nextVRon.Status = vron.StatusTerminated
						return
					}
					if len(output.Facts) > 0 && m.OnSaveMemory != nil {
						for _, fact := range output.Facts {
							m.OnSaveMemory(fact, "fact")
						}
						m.adjustSerotonin(15)
					}
					nextVRon.Status = vron.StatusTerminated
					m.resumeParent(nextVRon.ParentID, "Consolidation complete", nextVRon.ThreadCost)

				case vron.MethodQueryMemory:
					retrieved := ""
					if m.OnRetrieveLTM != nil {
						query := nextVRon.Context.PassedContext
						if query == "" {
							query = nextVRon.Context.STM
						}
						retrieved = m.OnRetrieveLTM(query)
					}
					nextVRon.Status = vron.StatusTerminated
					m.resumeParent(nextVRon.ParentID, retrieved, nextVRon.ThreadCost)

				case vron.MethodTest:
					output, err := m.Provider.GenerateStructured(ctx, prompt, nextVRon.Context)
					if err != nil {
						nextVRon.Status = vron.StatusTerminated
						return
					}
					res := output.Result
					if res == "" {
						res = "UNKNOWN"
					}
					if res == "PASS" || res == "CONFLICT" {
						m.adjustSerotonin(10)
						if m.OnSaveMemory != nil {
							m.OnSaveMemory(output.Reasoning, "test_result")
						}
					} else {
						m.adjustSerotonin(-10)
					}
					nextVRon.Status = vron.StatusTerminated
					m.resumeParent(nextVRon.ParentID, res, nextVRon.ThreadCost)

				case vron.MethodReact:
					output, err := m.Provider.GenerateStructured(ctx, prompt, nextVRon.Context)
					if err == nil && output.SerotoninDelta != 0 {
						m.adjustSerotonin(output.SerotoninDelta)
					}
					nextVRon.Status = vron.StatusTerminated

				case vron.MethodPlan:
					output, err := m.Provider.GenerateStructured(ctx, prompt, nextVRon.Context)
					if err == nil && len(output.Tasks) > 0 {
						utils.LogInfo("[Plan] VRon %s generated %d tasks", nextVRon.ID, len(output.Tasks))
						firstTask := output.Tasks[0]
						childCtx := vron.VRonContext{
							PassedContext: firstTask,
							Method:        vron.MethodPromptUser,
							Goal:          string(vron.MethodPromptUser),
							STM:           nextVRon.Context.STM,
						}
						_ = m.Spawn("", childCtx, nil)
					}
					nextVRon.Status = vron.StatusTerminated
					m.resumeParent(nextVRon.ParentID, "Planning complete", nextVRon.ThreadCost)

				case vron.MethodPromptUser:
					output, err := m.Provider.GenerateStructured(ctx, prompt, nextVRon.Context)
					if err == nil && output.Message != "" && m.OnRespond != nil {
						m.OnRespond(output.Message)
					}
					nextVRon.Status = vron.StatusTerminated

				case vron.MethodContextSwap:
					retrieved := ""
					if m.OnRetrieveLTM != nil {
						retrieved = m.OnRetrieveLTM(nextVRon.Context.STM)
						utils.LogInfo("[ContextSwap] Refreshed active memory context (%d chars retrieved)", len(retrieved))
					}
					nextVRon.Status = vron.StatusTerminated
					m.resumeParent(nextVRon.ParentID, retrieved, nextVRon.ThreadCost)

				default:
					output, err := m.Provider.GenerateStructured(ctx, prompt, nextVRon.Context)
					if err == nil && output.Response != "" && m.OnRespond != nil {
						m.OnRespond(output.Response)
					}
					nextVRon.Status = vron.StatusTerminated
					m.resumeParent(nextVRon.ParentID, output.Response, nextVRon.ThreadCost)
				}
			}()
		}
	}
}

// resumeParent wakes a suspended parent VRon by injecting the child's result.
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

	parent.Context.PassedContext = childResult
	parent.ThreadCost += childCost
	parent.Status = vron.StatusActive
	m.priorityQueue = append(m.priorityQueue, parent)
	m.mu.Unlock()

	utils.LogDebug("[InstanceManager] Parent %s resumed with result (%d chars)", parentID, len(childResult))
}
