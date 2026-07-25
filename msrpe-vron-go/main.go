package main

import (
	"context"
	"flag"
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

	// Start the background Queue loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.Manager.RunQueue(ctx)

	// Start the interactive Interface loop (Blocking)
	app.RunLoop(ctx)
}
