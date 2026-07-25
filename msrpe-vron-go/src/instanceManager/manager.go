package instanceManager

import (
	"context"
	"fmt"
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

// PendingVRon represents a VRon waiting in the queue.
type PendingVRon struct {
	ID         string
	ParentID   string
	Context    vron.VRonContext
	IsSuspended bool
	Instance   vron.VRon
}

// Manager controls the lifecycle, rate limits, and energy pool of all VRons.
type Manager struct {
	mu           sync.Mutex
	GlobalEnergy int
	TickCooldown time.Duration
	BaseTick     time.Duration
	SleepMode    SleepMode

	queue       []*PendingVRon
	suspended   map[string]*PendingVRon
	activeVRons int
}

// NewManager creates a new InstanceManager with default constraints.
func NewManager() *Manager {
	return &Manager{
		GlobalEnergy: 100,
		BaseTick:     4 * time.Second,
		TickCooldown: 4 * time.Second, // Starts equal to BaseTick
		SleepMode:    ModeActive,
		queue:        make([]*PendingVRon, 0),
		suspended:    make(map[string]*PendingVRon),
	}
}

// Spawn enqueues a new VRon for execution. It applies biological backpressure.
func (m *Manager) Spawn(parentID string, vronCtx vron.VRonContext, instance vron.VRon) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Biological backpressure: If energy is critically low, refuse to spawn background VRons
	if m.GlobalEnergy <= 10 && parentID != "" {
		return fmt.Errorf(vron.SysMsgFatigue)
	}

	// Cost scaling: Deduct energy for the spawn
	m.GlobalEnergy -= 2
	if m.GlobalEnergy < 0 {
		m.GlobalEnergy = 0
	}

	// Dynamic Cooldown logic: +2 seconds per spawn as energy drops
	// Formula: BaseTick + ((100 - Energy) / 10 * 2s)
	// Example: Energy 80 -> 4s + (2 * 2s) = 8s cooldown
	energyDeficit := (100 - m.GlobalEnergy) / 10
	m.TickCooldown = m.BaseTick + time.Duration(energyDeficit*2)*time.Second

	fmt.Printf("[InstanceManager] Spawned VRon. Energy: %d, New Cooldown: %v\n", m.GlobalEnergy, m.TickCooldown)

	v := &PendingVRon{
		ID:       fmt.Sprintf("vron-%d", time.Now().UnixNano()),
		ParentID: parentID,
		Context:  vronCtx,
		Instance: instance,
	}

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
			// Apply Tick Cooldown (protects API rate limits)
			time.Sleep(m.TickCooldown)

			m.mu.Lock()
			if m.SleepMode == ModeHibernation {
				m.mu.Unlock()
				time.Sleep(10 * time.Second) // Check less frequently in deep hibernation
				continue
			}

			if len(m.queue) == 0 {
				m.mu.Unlock()
				continue
			}

			// Pop the first pending VRon
			nextVRon := m.queue[0]
			m.queue = m.queue[1:]
			m.mu.Unlock()

			// Execute the VRon (this would block, representing LLM time)
			// For async pausing: If the VRon returns a "spawn_child" action,
			// we suspend it, put it in the suspended map, and spawn the child.
			// When the child finishes, we pull it from suspended and re-queue it.
			
			// Mock execution for skeleton:
			fmt.Printf("[InstanceManager] Executing VRon %s\n", nextVRon.ID)
			
			// Simulate Energy Burn during execution
			m.mu.Lock()
			m.GlobalEnergy -= 1 
			if m.GlobalEnergy < 0 {
				m.GlobalEnergy = 0
			}
			m.mu.Unlock()
		}
	}
}
