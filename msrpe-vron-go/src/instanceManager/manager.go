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
	Scores       vron.MindScores // Full 5-part biological mindstate (MA:UA:SE:OX:CO)
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
	OnRespond                 func(response string)
	OnRetrieveLTM             func(query string) string
	OnRetrieveLTMConsolidated func(query string) string
	OnSaveMemory              func(content, epType string)
	OnSaveSpecialEpisode      func(facts []string, epType string, mindState string)
	OnAddShortTermFact        func(fact string)
	OnSubconsciousTrigger     func()

	vronPool         chan *PendingVRon
	queue            []*PendingVRon
	priorityQueue    []*PendingVRon
	suspended        map[string]*PendingVRon
	activeVRons      int
	lastEnergyBurn   int
	lastActivity     time.Time
	lastSubconscious time.Time
	wakeChan         chan struct{}
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

func (m *Manager) getMindStateLocked() string {
	return m.Scores.String()
}

// GetMindState returns the formatted MA:UA:SE:OX:CO mindstate string.
func (m *Manager) GetMindState() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getMindStateLocked()
}

func (m *Manager) adjustMindScoresLocked(deltaMA, deltaUA, deltaSE, deltaOX, deltaCO float64) {
	m.Scores.MA += deltaMA
	if m.Scores.MA > 1.0 { m.Scores.MA = 1.0 }
	if m.Scores.MA < 0.0 { m.Scores.MA = 0.0 }

	m.Scores.UA += deltaUA
	if m.Scores.UA > 1.0 { m.Scores.UA = 1.0 }
	if m.Scores.UA < 0.0 { m.Scores.UA = 0.0 }

	m.Scores.SE += deltaSE
	if m.Scores.SE > 1.0 { m.Scores.SE = 1.0 }
	if m.Scores.SE < -1.0 { m.Scores.SE = -1.0 }

	m.Scores.OX += deltaOX
	if m.Scores.OX > 1.0 { m.Scores.OX = 1.0 }
	if m.Scores.OX < 0.0 { m.Scores.OX = 0.0 }

	m.Scores.CO += deltaCO
	if m.Scores.CO > 1.0 { m.Scores.CO = 1.0 }
	if m.Scores.CO < 0.0 { m.Scores.CO = 0.0 }

	m.SerotoninLevel = int(m.Scores.SE * 100)
}

// AdjustMindScores adjusts the 5 biological scores safely within bounds.
func (m *Manager) AdjustMindScores(deltaMA, deltaUA, deltaSE, deltaOX, deltaCO float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.adjustMindScoresLocked(deltaMA, deltaUA, deltaSE, deltaOX, deltaCO)
}

// NewManager creates a new InstanceManager with rules-driven constraints.
func NewManager() (*Manager, error) {
	config := envconfig.Load()

	provider, err := providers.NewInferenceProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize InferenceProvider: %v", err)
	}

	m := &Manager{
		GlobalEnergy: config.MaxGlobalEnergy,
		MaxEnergy:    config.MaxGlobalEnergy,
		Scores: vron.MindScores{
			MA: 0.90,
			UA: 0.30,
			SE: 0.00,
			OX: 0.50,
			CO: 0.10,
		},
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
		wakeChan:            make(chan struct{}, 1),
		lastActivity:        time.Now(),
	}

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
			drainLoop:
				for {
					select {
					case <-m.vronPool:
					default:
						break drainLoop
					}
				}
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
	m.lastActivity = time.Now()
	m.SleepMode = ModeActive

	// Replenish energy on user interaction if low so engine is never stuck
	criticalThreshold := int(float64(m.MaxEnergy) * m.FatigueThresholdPct)
	if m.GlobalEnergy <= criticalThreshold+20 {
		m.GlobalEnergy = int(float64(m.MaxEnergy) * 0.50)
		utils.LogInfo("[InstanceManager] Biological energy boosted to 50%% by user activity (Energy: %d)", m.GlobalEnergy)
	}
	m.mu.Unlock()
	m.AdjustMindScores(0.10, -0.05, 0, 0.05, -0.05)
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
					// Reserve cells for user chat: only trigger subconscious thoughts if cells are available
					if m.SleepMode == ModeUserIdle && m.OnSubconsciousTrigger != nil && !isExhausted && len(m.vronPool) > 3 {
						if time.Since(m.lastSubconscious) >= 30*time.Second {
							m.lastSubconscious = time.Now()
							triggerSubconscious = true
						}
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

func (m *Manager) adjustSerotonin(delta int) {
	m.AdjustMindScores(0, 0, float64(delta)/100.0, 0, 0)
}

// Spawn enqueues a new VRon for execution with an assigned Method.
func (m *Manager) Spawn(parentID string, vronCtx vron.VRonContext, instance vron.VRon) error {
	m.mu.Lock()

	criticalThreshold := int(float64(m.MaxEnergy) * m.FatigueThresholdPct)
	if m.GlobalEnergy <= criticalThreshold && parentID != "" {
		m.mu.Unlock()
		return fmt.Errorf(vron.SysMsgFatigue)
	}

	var v *PendingVRon
	select {
	case v = <-m.vronPool:
	default:
		// Priority allocation: dynamically create a cell for primary/user messages so chat is NEVER dropped!
		if parentID == "" {
			v = m.createEmptyVRon()
			utils.LogInfo("[InstanceManager] Dynamically allocated cell %s for user message", v.ID)
		} else {
			m.mu.Unlock()
			utils.LogInfo("[InstanceManager] VRon pool exhausted for background task. Waiting for batch regeneration...")
			return fmt.Errorf("VRon pool exhausted, rate limit hit")
		}
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

	utils.LogInfo("[InstanceManager] VRon Created | ID: %s | Method: %s | Depth: %d | Cost: %d | Energy: %d", v.ID, v.Method, v.ThreadDepth, v.ThreadCost, m.GlobalEnergy)

	m.queue = append(m.queue, v)

	select {
	case m.wakeChan <- struct{}{}:
	default:
	}

	m.mu.Unlock()
	return nil
}

// RunQueue starts the background processing loop.
func (m *Manager) RunQueue(ctx context.Context) {
	utils.LogInfo("[InstanceManager] RunQueue loop started")
	for {
		m.mu.Lock()
		hasPending := len(m.queue) > 0 || len(m.priorityQueue) > 0
		cooldown := m.TickCooldown
		sleepMode := m.SleepMode
		sleepMult := 2.5
		if m.CostProvider != nil {
			sleepMult = m.CostProvider.GetHibernationSleepMult()
		}
		m.mu.Unlock()

		// Only sleep if there are no pending VRons to process
		if !hasPending {
			targetCooldown := cooldown
			if sleepMode == ModeHibernation {
				targetCooldown = time.Duration(float64(m.BaseTick.Milliseconds())*sleepMult) * time.Millisecond
			}

			// Drain any stale wake tokens before waiting
		drainWake:
			for {
				select {
				case <-m.wakeChan:
				default:
					break drainWake
				}
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(targetCooldown):
			case <-m.wakeChan:
			}
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

		m.adjustMindScoresLocked(-0.01, -0.01, 0, -0.005, -0.01)

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
		nextVRon.Context.MindState = m.getMindStateLocked()
		m.mu.Unlock()

		utils.LogInfo("[InstanceManager] Executing VRon %s | Method: %s | Depth: %d | MindState: %s", nextVRon.ID, nextVRon.Method, nextVRon.ThreadDepth, nextVRon.Context.MindState)

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

				if nextVRon.Status == vron.StatusTerminated {
					nextVRon.Status = vron.StatusIdle
					nextVRon.ParentID = ""
					nextVRon.WaitingForID = ""
					nextVRon.Context = vron.VRonContext{}
					nextVRon.Instance = nil
					select {
					case m.vronPool <- nextVRon:
					default:
					}
				}
				m.mu.Unlock()
			}()

			utils.LogInfo("[InstanceManager] Entering func() for VRon %s", nextVRon.ID)
			nextVRon.Context.EnergyLevel = m.GlobalEnergy
			nextVRon.Context.SerotoninLevel = m.SerotoninLevel
			nextVRon.Context.MindState = m.GetMindState()

			prompt := vron.GetMethodPrompt(nextVRon.Method)

			switch nextVRon.Method {
			case vron.MethodRespond:
				utils.LogInfo("[InstanceManager] Sending LLM request for VRon %s (Method: %s)...", nextVRon.ID, nextVRon.Method)
				output, err := m.Provider.GenerateStructured(ctx, prompt, nextVRon.Context)
				if err != nil {
					utils.LogInfo("[VRon-FAIL] %s execution error: %v", nextVRon.ID, err)
					m.adjustSerotonin(-10)
					nextVRon.Status = vron.StatusTerminated
					return
				}
				utils.LogInfo("[InstanceManager] VRon %s LLM request completed successfully.", nextVRon.ID)

				if len(output.Facts) > 0 && m.OnSaveSpecialEpisode != nil {
					m.OnSaveSpecialEpisode(output.Facts, "fact", m.GetMindState())
				}

				if nextVRon.Context.PassedContext == "" && (output.NeedMemory || output.MemoryQuery != "") && output.MemoryQuery != "" {
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
					m.mu.Lock()
					delete(m.suspended, nextVRon.ID)
					nextVRon.Status = vron.StatusActive
					m.mu.Unlock()
				} else if nextVRon.Context.PassedContext == "" && (output.NeedTest || output.TestQuery != "") && output.TestQuery != "" {
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

			case vron.MethodUpdateMemory:
				output, err := m.Provider.GenerateStructured(ctx, prompt, nextVRon.Context)
				if err != nil {
					utils.LogInfo("[VRon-FAIL] %s execution error: %v", nextVRon.ID, err)
					nextVRon.Status = vron.StatusTerminated
					return
				}
				threshold := 80
				if nextVRon.Context.SerotoninLevel < 0 {
					threshold = 95
				}
				if output.Confidence < threshold {
					utils.LogInfo("[InstanceManager] VRon %s update_memory low confidence (%d < %d). Blocked.", nextVRon.ID, output.Confidence, threshold)
					m.adjustSerotonin(-10)
					nextVRon.Status = vron.StatusTerminated
					return
				}
				if len(output.Facts) > 0 && m.OnSaveSpecialEpisode != nil {
					m.OnSaveSpecialEpisode(output.Facts, "fact", m.GetMindState())
					m.adjustSerotonin(15)
				}
				nextVRon.Status = vron.StatusTerminated
				m.resumeParent(nextVRon.ParentID, "Fact episode successfully saved to memory.", nextVRon.ThreadCost)

			case vron.MethodConsolidate:
				output, err := m.Provider.GenerateStructured(ctx, prompt, nextVRon.Context)
				if err != nil {
					utils.LogInfo("[VRon-FAIL] %s execution error: %v", nextVRon.ID, err)
					nextVRon.Status = vron.StatusTerminated
					return
				}
				if len(output.Facts) > 0 && m.OnSaveMemory != nil {
					summaryContent := strings.Join(output.Facts, " | ")
					m.OnSaveMemory(summaryContent, "summary")
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
					if retrieved != "" && m.OnAddShortTermFact != nil {
						m.OnAddShortTermFact(retrieved)
					}
				}
				nextVRon.Status = vron.StatusTerminated
				m.resumeParent(nextVRon.ParentID, retrieved, nextVRon.ThreadCost)

			case vron.MethodTest:
				output, err := m.Provider.GenerateStructured(ctx, prompt, nextVRon.Context)
				if err != nil {
					utils.LogInfo("[VRon-FAIL] %s execution error: %v", nextVRon.ID, err)
					nextVRon.Status = vron.StatusTerminated
					return
				}
				res := output.Result
				if res == "" {
					res = "UNKNOWN"
				}
				if res == "PASS" || res == "CONFLICT" {
					if res == "CONFLICT" {
						m.AdjustMindScores(0, 0, 0, 0, 0.30)
					} else {
						m.adjustSerotonin(10)
					}
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
				if err == nil {
					if output.SerotoninDelta != 0 {
						m.adjustSerotonin(output.SerotoninDelta)
					}
					respText := output.Response
					if respText == "" {
						respText = output.Message
					}
					if respText != "" && m.OnRespond != nil {
						m.OnRespond(respText)
						utils.LogInfo("[InstanceManager] VRon %s sent proactive message: %s", nextVRon.ID, respText)
					}
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
				if m.OnRetrieveLTMConsolidated != nil {
					retrieved = m.OnRetrieveLTMConsolidated(nextVRon.Context.STM)
					utils.LogInfo("[ContextSwap] Refreshed active memory context with consolidated episodes (%d chars retrieved)", len(retrieved))
				} else if m.OnRetrieveLTM != nil {
					retrieved = m.OnRetrieveLTM(nextVRon.Context.STM)
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

	select {
	case m.wakeChan <- struct{}{}:
	default:
	}

	m.mu.Unlock()

	utils.LogInfo("[InstanceManager] Parent %s resumed with result (%d chars)", parentID, len(childResult))
}
