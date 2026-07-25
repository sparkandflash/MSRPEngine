package main

import (
	"context"
	"fmt"
	"msrpe-vron-go/src/instanceManager"
	"msrpe-vron-go/src/ruleEngine"
	"time"
)

func main() {
	fmt.Println("Booting VRON-V1 Engine...")

	// 1. Initialize the Instance Manager (Scheduler)
	manager := instanceManager.NewManager()

	// 2. Initialize the Rule Engine (Reflexes)
	dispatcher := &ruleEngine.ReflexDispatcher{
		Manager: manager,
	}

	// 3. Start the background Queue loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go manager.RunQueue(ctx)

	// 4. Simulate a burst of user inputs to watch Energy Drain and Backpressure
	fmt.Println("\n--- Simulating Burst Traffic ---")
	
	for i := 1; i <= 6; i++ {
		fmt.Printf("\n[Simulation] Firing User Message %d...\n", i)
		dispatcher.OnUserMessage(fmt.Sprintf("Hello VRON, this is message %d", i))
		
		// Fire them quickly to watch the queue fill up and cooldowns scale
		time.Sleep(500 * time.Millisecond)
	}

	// Wait long enough to watch the RunQueue slowly pop them off at the rate limits
	fmt.Println("\n[Simulation] Waiting to observe Dynamic Cooldown popping...")
	time.Sleep(30 * time.Second)
	
	fmt.Println("\nEngine Shutting Down.")
}
