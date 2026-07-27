package main

import (
	"context"
	"flag"
	"msrpe-vron-go/src/envconfig"
	"msrpe-vron-go/src/interface"
	"msrpe-vron-go/src/utils"
)

func main() {
	debugFlag := flag.Bool("debug", false, "Enable verbose debugging output")
	flag.Parse()

	if *debugFlag {
		utils.DebugMode = true
		utils.LogDebug("Debug mode enabled.")
	}

	app, err := interfaceUI.NewAppCore()
	if err != nil {
		utils.LogInfo("FATAL: Failed to boot engine: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start VRon pool regeneration (batch refill every VRonPoolRefillRate, e.g. 60s)
	config := envconfig.Load()
	app.Manager.StartPoolRegen(ctx, config.VRonPoolRefillRate)

	// Start periodic background schedulers (ContextSwap, etc.)
	if app.Scheduler != nil {
		app.Scheduler.StartBackgroundSchedulers(ctx)
	}

	// Start the interactive Interface loop (Blocking)
	app.RunLoop(ctx)
}
