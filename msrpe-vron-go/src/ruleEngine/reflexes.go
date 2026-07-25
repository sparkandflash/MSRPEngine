package ruleEngine

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
	"msrpe-vron-go/src/instanceManager"
	"msrpe-vron-go/src/utils"
	"msrpe-vron-go/src/vron"
)

// RuleConfig maps to the rules.yaml structure
type RuleConfig struct {
	EnergyDrainRatios struct {
		SpawnCost          float64 `yaml:"spawn_cost"`
		ExecuteCost        float64 `yaml:"execute_cost"`
		PassiveCostPerVRon float64 `yaml:"passive_cost_per_vron"`
	} `yaml:"energy_drain_ratios"`
	EnergyRegenRatios struct {
		Active      float64 `yaml:"active"`
		UserIdle    float64 `yaml:"user_idle"`
		Hibernation float64 `yaml:"hibernation"`
	} `yaml:"energy_regen_ratios"`
	SchedulerRatios struct {
		HibernationSleepMultiplier float64 `yaml:"hibernation_sleep_multiplier"`
		CooldownPenaltyFactor      float64 `yaml:"cooldown_penalty_factor"`
	} `yaml:"scheduler_ratios"`
}

// ReflexDispatcher acts as the baseline nervous system.
type ReflexDispatcher struct {
	Manager *instanceManager.Manager

	// Ratios
	SpawnCostRatio          float64
	ExecuteCostRatio        float64
	PassiveCostRatio        float64
	ActiveRegenRatio        float64
	UserIdleRegenRatio      float64
	HibernationRegenRatio   float64
	HibernationSleepMult    float64
	CooldownPenaltyFactor   float64
}

func (r *ReflexDispatcher) GetSpawnCostRatio() float64 { return r.SpawnCostRatio }
func (r *ReflexDispatcher) GetExecuteCostRatio() float64 { return r.ExecuteCostRatio }
func (r *ReflexDispatcher) GetPassiveCostRatio() float64 { return r.PassiveCostRatio }
func (r *ReflexDispatcher) GetActiveRegenRatio() float64 { return r.ActiveRegenRatio }
func (r *ReflexDispatcher) GetUserIdleRegenRatio() float64 { return r.UserIdleRegenRatio }
func (r *ReflexDispatcher) GetHibernationRegenRatio() float64 { return r.HibernationRegenRatio }
func (r *ReflexDispatcher) GetHibernationSleepMult() float64 { return r.HibernationSleepMult }
func (r *ReflexDispatcher) GetCooldownPenaltyFactor() float64 { return r.CooldownPenaltyFactor }

// NewReflexDispatcher creates the Rule Engine and loads dynamic costs from YAML.
func NewReflexDispatcher() *ReflexDispatcher {
	rd := &ReflexDispatcher{
		SpawnCostRatio:        0.02,
		ExecuteCostRatio:      0.01,
		PassiveCostRatio:      0.01,
		ActiveRegenRatio:      0.01,
		UserIdleRegenRatio:    0.02,
		HibernationRegenRatio: 0.05,
		HibernationSleepMult:  2.5,
		CooldownPenaltyFactor: 0.5,
	}
	rd.loadYAML()
	return rd
}

func (r *ReflexDispatcher) loadYAML() {
	yamlPath := "src/ruleEngine/rules.yaml"
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		utils.LogInfo("[RuleEngine] Failed to read rules.yaml, using default costs: %v", err)
		return
	}

	var config RuleConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		utils.LogInfo("[RuleEngine] Failed to parse rules.yaml, using default costs: %v", err)
		return
	}

	// Only overwrite if the YAML actually set a non-zero value.
	// This preserves the struct defaults for any key not defined in rules.yaml.
	if config.EnergyDrainRatios.SpawnCost > 0 {
		r.SpawnCostRatio = config.EnergyDrainRatios.SpawnCost
	}
	if config.EnergyDrainRatios.ExecuteCost > 0 {
		r.ExecuteCostRatio = config.EnergyDrainRatios.ExecuteCost
	}
	if config.EnergyDrainRatios.PassiveCostPerVRon > 0 {
		r.PassiveCostRatio = config.EnergyDrainRatios.PassiveCostPerVRon
	}
	if config.EnergyRegenRatios.Active > 0 {
		r.ActiveRegenRatio = config.EnergyRegenRatios.Active
	}
	if config.EnergyRegenRatios.UserIdle > 0 {
		r.UserIdleRegenRatio = config.EnergyRegenRatios.UserIdle
	}
	if config.EnergyRegenRatios.Hibernation > 0 {
		r.HibernationRegenRatio = config.EnergyRegenRatios.Hibernation
	}
	if config.SchedulerRatios.HibernationSleepMultiplier > 0 {
		r.HibernationSleepMult = config.SchedulerRatios.HibernationSleepMultiplier
	}
	if config.SchedulerRatios.CooldownPenaltyFactor > 0 {
		r.CooldownPenaltyFactor = config.SchedulerRatios.CooldownPenaltyFactor
	}

	utils.LogInfo("[RuleEngine] Loaded ratios from rules.yaml")
}

// OnUserMessage is a reflex triggered when the interface receives a chat message.
func (r *ReflexDispatcher) OnUserMessage(message string) {
	utils.LogInfo("[RuleEngine] Reflex Triggered: User Message Received.")
	r.Manager.RecordUserActivity()

	// Assemble the initial context (STM)
	ctx := vron.VRonContext{
		STM:             fmt.Sprintf("User says: %s", message),
		LTM:             "", // Root VRon must spawn a child to retrieve this if needed
		EnergyLevel:     r.Manager.GetEnergy(),          // BUG FIX #1: thread-safe read
		ConsumptionRate: r.Manager.GetConsumptionRate(), // BUG FIX #1: thread-safe read
		PassedContext:   "",
	}

	// Spawn the Root VRon. Notice we pass an empty parentID because this is a root.
	err := r.Manager.Spawn("", ctx, nil)
	if err != nil {
		utils.LogInfo("[RuleEngine] Reflex failed to spawn VRon: %v", err)
	}
}
