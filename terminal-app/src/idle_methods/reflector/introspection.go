package reflector

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"terminal-app/src/idle_methods/consolidation"
	"terminal-app/src/prompts"
	"terminal-app/src/summariser"
	"terminal-app/src/utils"
)

// Introspect reads a target episode, sends it to the summariser agent (using the introspection prompt)
// to generate alternative responses, and saves the reflection to disk.
func Introspect(episodeID string) error {
	// 1. Read original episode
	episodesDir := utils.ResolvePath(filepath.Join("Context", "episodes"))
	episodePath := filepath.Join(episodesDir, fmt.Sprintf("%s.json", episodeID))
	
	data, err := os.ReadFile(episodePath)
	if err != nil {
		return fmt.Errorf("failed to read episode %s: %w", episodeID, err)
	}

	var episode consolidation.Episode
	if err := json.Unmarshal(data, &episode); err != nil {
		return fmt.Errorf("failed to parse episode %s: %w", episodeID, err)
	}

	if len(episode.MessageIDs) == 0 {
		return fmt.Errorf("episode %s has no messages to introspect", episodeID)
	}

	// Extract SessionID from EpisodeID (format: <SessionID>_ep_<timestamp>)
	parts := strings.SplitN(episodeID, "_ep_", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid episode ID format: %s", episodeID)
	}
	sessionID := parts[0]

	// Read full history to resolve MessageIDs
	historyPath := utils.ResolvePath(filepath.Join("Context", "conversationHistory", fmt.Sprintf("%s.json", sessionID)))
	historyData, err := os.ReadFile(historyPath)
	if err != nil {
		return fmt.Errorf("failed to read conversation history %s: %w", sessionID, err)
	}

	// We need to define or import the Message struct. We can import consolidator and use consolidator.Message.
	var fullHistory []map[string]interface{}
	if err := json.Unmarshal(historyData, &fullHistory); err != nil {
		return fmt.Errorf("failed to parse conversation history: %w", err)
	}

	// Build a map of msgID -> map for quick lookup
	msgMap := make(map[string]map[string]interface{})
	for _, msg := range fullHistory {
		if id, ok := msg["id"].(string); ok {
			msgMap[id] = msg
		}
	}

	// 2. Format messages for the summariser
	var convBuilder strings.Builder
	for _, msgID := range episode.MessageIDs {
		if msg, ok := msgMap[msgID]; ok {
			author, _ := msg["author"].(string)
			content, _ := msg["content"].(string)
			convBuilder.WriteString(fmt.Sprintf("%s: %s\n", author, content))
		}
	}

	// 3. Call SummariserAgent with the Introspection Prompt
	agent := summariser.NewSummariserAgent()
	
	// We need a way to override the system instruction. The SummariserAgent currently loads from config,
	// but we can modify its config temporarily or we can just call its unexported callLLM if we modify it.
	// Since SummariserAgent.Summarise defaults to prompts.GetConsolidationPrompt(), we should probably
	// add a SummariseWithPrompt method or just temporarily change the agent's config.
	// We will temporarily mutate the agent config (if it's exported, but it's not).
	// To avoid modifying summariser package heavily, we can add a new method to summariser if needed,
	// or we can just re-implement a small call here.
	
	// Wait, let's look at summariser package. `SummariserAgent` has `Summarise(ctx, text)`. 
	// We should just use a direct LLM call or modify `summariser.go` to support custom prompts.
	// For now, I'll assume we can call an updated `SummariseWithPrompt` in `summariser`.
	rawJSON, err := agent.SummariseWithPrompt(context.Background(), convBuilder.String(), prompts.GetIntrospectionPrompt())
	if err != nil {
		return fmt.Errorf("introspection LLM call failed: %w", err)
	}

	var llmResp consolidation.LLMResponse
	if err := json.Unmarshal([]byte(rawJSON), &llmResp); err != nil {
		return fmt.Errorf("failed to parse introspection JSON: %w (raw: %s)", err, rawJSON)
	}

	// 4. Save Reflection JSON
	reflectionsDir := filepath.Join(episodesDir, "reflections")
	if err := os.MkdirAll(reflectionsDir, 0755); err != nil {
		return fmt.Errorf("failed to create reflections directory: %w", err)
	}

	reflectionID := fmt.Sprintf("%s_reflection", episodeID)
	reflectionPath := filepath.Join(reflectionsDir, fmt.Sprintf("%s.json", reflectionID))
	
	reflectionData := map[string]interface{}{
		"original_episode_id": episodeID,
		"reflection_summary":  llmResp.Summary,
		"keywords":            llmResp.Keywords,
		"alternative_strategy": llmResp.Conclusion,
	}
	
	rBytes, err := json.MarshalIndent(reflectionData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize reflection: %w", err)
	}
	
	if err := os.WriteFile(reflectionPath, rBytes, 0644); err != nil {
		return fmt.Errorf("failed to write reflection file: %w", err)
	}

	// 5. Append to reflections.csv
	csvPath := filepath.Join(episodesDir, "reflections.csv")
	csvFile, err := os.OpenFile(csvPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open reflections.csv: %w", err)
	}
	defer csvFile.Close()

	writer := csv.NewWriter(csvFile)
	
	// Check if file is empty to write header
	stat, _ := csvFile.Stat()
	if stat.Size() == 0 {
		writer.Write([]string{"original_episodeid", "keywords", "reflectionid"})
	}
	
	writer.Write([]string{
		episodeID,
		strings.Join(llmResp.Keywords, ", "),
		reflectionID,
	})
	writer.Flush()

	return writer.Error()
}
