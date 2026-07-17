package escalator

import (
	"context"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// Scheduler runs the rule engine on a ticker and emits events.
type Scheduler struct {
	Engine    *RuleEngine
	EventChan chan EventType
	GetMindState func() string
	HasUnconsolidated func() bool
}

// NewScheduler creates a new scheduler instance.
func NewScheduler(getMindState func() string, hasUnconsolidated func() bool) *Scheduler {
	return &Scheduler{
		Engine:            NewRuleEngine(),
		EventChan:         make(chan EventType, 10),
		GetMindState:      getMindState,
		HasUnconsolidated: hasUnconsolidated,
	}
}

// Run starts the 5-second background ticker.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			currentMindState := s.GetMindState()
			s.Engine.UpdateHeartrate(currentMindState)

			// Determine if an event should be emitted
			evt := s.Engine.EvaluateState(currentMindState, s.HasUnconsolidated())
			
			if evt != EventNothing {
				// Parse MA for skip logic
				var ma float64
				parts := strings.Split(currentMindState, ":")
				if len(parts) > 0 {
					ma, _ = strconv.ParseFloat(parts[0], 64)
				}

				// If model attention is too low (<0.20), skip firing events randomly (33% chance)
				if ma < 0.20 && rand.Float64() < 0.3333 {
					// Event generated but suppressed due to low attention
					continue
				}

				// Emit event and acknowledge immediately so cooldowns start
				s.Engine.AcknowledgeEvent(evt)
				
				select {
				case s.EventChan <- evt:
					// Sent successfully
				default:
					// Channel full, drop event to prevent blocking
				}
			}
		}
	}
}
