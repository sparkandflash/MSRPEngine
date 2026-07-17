package cli

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"lyra/consolidator"
	"lyra/escalator"
	"lyra/idle_methods/consolidation"
	episode_memory "lyra/idle_methods/episode_memory"
	"lyra/idle_methods/reflector"
	"lyra/reactor"
	"lyra/responder"
)

// Run starts the interactive chat interface for Lyra.
func Run() {
	// Initialize the responder agent from environment configuration
	resp, err := responder.NewResponderFromEnv()
	if err != nil {
		fmt.Printf("system error: failed to initialize responder: %v\n", err)
		os.Exit(1)
	}

	// Initialize the reactor agent
	reactorAgent := reactor.NewReactorAgent()

	// ── Reactor STM ──────────────────────────────────────────────────────────
	// LYRA_MAX_WORKING_MEMORY_CHARS controls the reactor's short-term memory window (default 2000).
	reactorMaxChars := 2000
	if limitStr := os.Getenv("LYRA_MAX_WORKING_MEMORY_CHARS"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && limit > 0 {
			reactorMaxChars = limit
		}
	}
	reactorSTM := consolidator.NewSTMmanager(reactorMaxChars)

	// ── Responder STM ────────────────────────────────────────────────────────
	// LYRA_RESPONDER_STM_CHARS controls the responder's short-term memory window (default 2000).
	responderMaxChars := 2000
	if limitStr := os.Getenv("LYRA_RESPONDER_STM_CHARS"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && limit > 0 {
			responderMaxChars = limit
		}
	}
	responderSTM := consolidator.NewSTMmanager(responderMaxChars)

	// ── Episode Memory Manager ────────────────────────────────────────────────
	// LYRA_EPISODE_MEMORY_CHARS controls the runtime episode pool's character budget (default 2000).
	episodeMgr := episode_memory.LoadEpisodeMemoryManagerFromEnv()

	// Initialize long-term conversation history store
	historyMgr, err := consolidator.NewHistoryManager()
	if err != nil {
		fmt.Printf("system error: failed to initialize history manager: %v\n", err)
		os.Exit(1)
	}

	mindState := "0.90:0.30:0.50:0.70"

	// State for rule engine integration
	hasUnconsolidated := false
	inputLockedUntil := time.Time{}

	// Initialize Escalator (Scheduler and Rule Engine)
	sched := escalator.NewScheduler(
		func() string { return mindState },
		func() bool { return hasUnconsolidated },
	)
	go sched.Run(context.Background())

	// Background input reader
	inputChan := make(chan string)
	scanner := bufio.NewScanner(os.Stdin)
	go func() {
		for scanner.Scan() {
			inputChan <- scanner.Text()
		}
	}()

	fmt.Print("user: ")
	
	for {
		select {
		case evt := <-sched.EventChan:
			switch evt {
			case escalator.EventConsolidate:
				newEpisodes, err := consolidation.Consolidate(historyMgr)
				if err == nil {
					for _, ep := range newEpisodes {
						episodeMgr.Push(episode_memory.EpisodeSummary{
							ID:            ep.ID,
							Summary:       ep.Summary,
							Keywords:      ep.Keywords,
							PeakMindState: ep.PeakMindState,
							Conclusion:    ep.Conclusion,
						})
					}
					hasUnconsolidated = false
				}
			case escalator.EventReflect:
				activeEps := episodeMgr.GetActive()
				episodes := make([]responder.EpisodeSummary, len(activeEps))
				for i, ep := range activeEps {
					episodes[i] = responder.EpisodeSummary{ID: ep.ID, Summary: ep.Summary, Keywords: ep.Keywords, PeakMindState: ep.PeakMindState, Conclusion: ep.Conclusion}
				}
				matchedIDs, _ := reflector.Reflect(mindState, episodes)
				for _, id := range matchedIDs {
					_ = episodeMgr.LoadFromDisk(id)
				}
			case escalator.EventIntrospect:
				activeEps := episodeMgr.GetActive()
				if len(activeEps) > 0 {
					_ = reflector.Introspect(activeEps[0].ID)
				}
			case escalator.EventProactiveMessage:
				fmt.Print("\r\033[K[Input Locked...] ")
				inputLockedUntil = time.Now().Add(3 * time.Second)

				ctx := context.Background()
				activeEps := episodeMgr.GetActive()
				episodes := make([]responder.EpisodeSummary, len(activeEps))
				for i, ep := range activeEps {
					episodes[i] = responder.EpisodeSummary{ID: ep.ID, Summary: ep.Summary, Keywords: ep.Keywords, PeakMindState: ep.PeakMindState, Conclusion: ep.Conclusion}
				}

				reply, usefulEpisodeID, err := resp.RespondProactive(ctx, mindState, responderSTM.GetNoFlags(), episodes)
				if err == nil {
					if usefulEpisodeID != "" {
						episodeMgr.MarkUseful(usefulEpisodeID)
					}
					
					fmt.Print("\r\033[K") // Clear input lock text
					fmt.Printf("lyra: %s\n", reply)
					
					_ = historyMgr.Save("assistant", reply, mindState)
					responderSTM.Update("assistant", reply)
					reactorSTM.Update("assistant", reply)
					hasUnconsolidated = true

					if respState, err := reactorAgent.React(ctx, reactorSTM.Get()); err == nil {
						mindState = fmt.Sprintf("%.2f:%.2f:%.2f:%.2f", respState.ModelAttention, respState.NegativeEmotion, respState.PositiveEmotion, respState.UserAttention)
					}
				}
				
				time.Sleep(3 * time.Second)
				fmt.Print("\r\033[Kuser: ")
			}

		case rawInput := <-inputChan:
			if time.Now().Before(inputLockedUntil) {
				// Discard input during lock
				continue
			}

			input := strings.TrimSpace(rawInput)
			if input == "" {
				fmt.Print("user: ")
				continue
			}

			if input == ">>debug" {
				fmt.Printf("debug: mindstate: %s | HR: %.1f\n", mindState, sched.Engine.Heartrate)
				fmt.Printf("debug: active episodes: %d | pinned: %q\n", len(episodeMgr.GetActive()), episodeMgr.GetPinnedID())
				fmt.Print("user: ")
				continue
			} else if strings.HasPrefix(input, ">>mindstate ") {
				valStr := strings.TrimSpace(strings.TrimPrefix(input, ">>mindstate "))
				var ma, ne, pe, ua float64
				_, err := fmt.Sscanf(valStr, "%f:%f:%f:%f", &ma, &ne, &pe, &ua)
				if err != nil || ma < 0.0 || ma > 1.0 || ne < 0.0 || ne > 1.0 || pe < 0.0 || pe > 1.0 || ua < 0.0 || ua > 1.0 {
					fmt.Println("debug: error: mindstate must be four floats (0.0 to 1.0) separated by colons (e.g. 0.9:0.3:0.5:0.7).")
				} else {
					mindState = fmt.Sprintf("%.2f:%.2f:%.2f:%.2f", ma, ne, pe, ua)
					fmt.Printf("debug: mindstate updated to %s.\n", mindState)
				}
				fmt.Print("user: ")
				continue
			} else if input == ">>consolidate" {
				newEpisodes, err := consolidation.Consolidate(historyMgr)
				if err != nil {
					fmt.Printf("debug: error: consolidation failed: %v\n", err)
				} else {
					for _, ep := range newEpisodes {
						episodeMgr.Push(episode_memory.EpisodeSummary{
							ID:            ep.ID,
							Summary:       ep.Summary,
							Keywords:      ep.Keywords,
							PeakMindState: ep.PeakMindState,
							Conclusion:    ep.Conclusion,
						})
					}
					hasUnconsolidated = false
					fmt.Printf("debug: consolidation completed successfully. %d episode(s) added.\n", len(newEpisodes))
				}
				fmt.Print("user: ")
				continue
			} else if input == ">>reflect" {
				activeEps := episodeMgr.GetActive()
				episodes := make([]responder.EpisodeSummary, len(activeEps))
				for i, ep := range activeEps {
					episodes[i] = responder.EpisodeSummary{ID: ep.ID, Summary: ep.Summary, Keywords: ep.Keywords, PeakMindState: ep.PeakMindState, Conclusion: ep.Conclusion}
				}
				matchedIDs, err := reflector.Reflect(mindState, episodes)
				if err != nil {
					fmt.Printf("debug: error: reflection failed: %v\n", err)
				} else {
					loaded := 0
					for _, id := range matchedIDs {
						if err := episodeMgr.LoadFromDisk(id); err == nil {
							loaded++
						}
					}
					fmt.Printf("debug: reflection completed. Found %d matching episodes, loaded %d into active memory.\n", len(matchedIDs), loaded)
				}
				fmt.Print("user: ")
				continue
			} else if strings.HasPrefix(input, ">>introspect ") {
				episodeID := strings.TrimSpace(strings.TrimPrefix(input, ">>introspect "))
				if err := reflector.Introspect(episodeID); err != nil {
					fmt.Printf("debug: error: introspection failed: %v\n", err)
				} else {
					fmt.Printf("debug: introspection completed for %s. Reflection saved.\n", episodeID)
				}
				fmt.Print("user: ")
				continue
			} else if input == "exit" || input == "quit" {
				fmt.Println("lyra: goodbye!")
				return
			}

			sched.Engine.OnUserMessage(mindState)
			hasUnconsolidated = true

			ctx := context.Background()
			
			// 3-second minimum delay + "thinking..." indicator
			startTime := time.Now()
			done := make(chan bool)
			go func() {
				fmt.Print("lyra: thinking")
				ticker := time.NewTicker(500 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-done:
						fmt.Print("\r\033[K") // clear the thinking line
						return
					case <-ticker.C:
						fmt.Print(".")
					}
				}
			}()

			// Save user message to long-term history
			_ = historyMgr.Save("user", input, mindState)
			// Update both STMs
			reactorSTM.Update("user", input)
			responderSTM.Update("user", input)

			var currentMA float64
			fmt.Sscanf(mindState, "%f:", &currentMA)

			// Skip logic: if MA < 0.20, 1/3 chance to skip processing
			if currentMA < 0.20 && rand.Float64() < 0.3333 {
				time.Sleep(time.Until(startTime.Add(3 * time.Second)))
				done <- true
				
				reply := "no response"
				fmt.Printf("lyra: %s\n", reply)
				_ = historyMgr.Save("assistant", reply, mindState)
				responderSTM.Update("assistant", reply)
				reactorSTM.Update("assistant", reply)
				fmt.Print("user: ")
				continue
			}

			// Invoke reactor agent to determine mindstate after user input
			if respState, err := reactorAgent.React(ctx, reactorSTM.Get()); err == nil {
				mindState = fmt.Sprintf("%.2f:%.2f:%.2f:%.2f", respState.ModelAttention, respState.NegativeEmotion, respState.PositiveEmotion, respState.UserAttention)
			}

			// Build the episode summaries from the active episode pool
			activeEps := episodeMgr.GetActive()
			episodes := make([]responder.EpisodeSummary, len(activeEps))
			for i, ep := range activeEps {
				episodes[i] = responder.EpisodeSummary{ID: ep.ID, Summary: ep.Summary, Keywords: ep.Keywords, PeakMindState: ep.PeakMindState, Conclusion: ep.Conclusion}
			}

			// Respond using responder's clean STM (no stored flags) + active episodes
			reply, usefulEpisodeID, err := resp.Respond(ctx, input, mindState, responderSTM.GetNoFlags(), episodes)
			if err != nil {
				done <- true
				fmt.Printf("lyra: error: failed to generate response: %v\n", err)
			} else {
				// If the model identified a useful episode, pin it to prevent eviction
				if usefulEpisodeID != "" {
					episodeMgr.MarkUseful(usefulEpisodeID)
				}

				// Invoke reactor agent to determine mindstate after assistant response
				if respState, err := reactorAgent.React(ctx, reactorSTM.Get()); err == nil {
					mindState = fmt.Sprintf("%.2f:%.2f:%.2f:%.2f", respState.ModelAttention, respState.NegativeEmotion, respState.PositiveEmotion, respState.UserAttention)
				}

				// Ensure at least 3 seconds have passed
				time.Sleep(time.Until(startTime.Add(3 * time.Second)))
				done <- true
				
				// Save assistant response to long-term history and responder STM
				_ = historyMgr.Save("assistant", reply, mindState)
				responderSTM.Update("assistant", reply)
				reactorSTM.Update("assistant", reply)

				fmt.Printf("lyra: %s\n", reply)
			}
			
			fmt.Print("user: ")
		}
	}
}
