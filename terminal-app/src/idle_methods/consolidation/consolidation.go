package consolidation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"terminal-app/src/consolidator"
	"terminal-app/src/embedder"
	"terminal-app/src/summariser"
	"terminal-app/src/utils"
)

// Episode represents the JSON structure of a consolidated episodic memory.
type Episode struct {
	ID            string   `json:"id"`
	Summary       string   `json:"summary"`
	PeakMindState string   `json:"peak_mindstate"`
	Conclusion    string   `json:"conclusion"`
	MessageIDs    []string `json:"message_ids"`
}

// LLMResponse matches the structured JSON expected from the Summariser LLM.
type LLMResponse struct {
	Summary    string   `json:"summary"`
	Conclusion string   `json:"conclusion"`
}

// EpisodeSummary is a lightweight, metadata-only view of an episode returned after consolidation.
// It contains no raw message data — only enough for the runtime episode memory manager.
type EpisodeSummary struct {
	ID            string   `json:"id"`
	Summary       string   `json:"summary"`
	PeakMindState string   `json:"peak_mindstate"`
	Conclusion    string   `json:"conclusion"`
}

// Consolidate reads unsaved messages from history, groups them by character length,
// calls the summariser agent to generate metadata, saves them to episode JSON/CSV files,
// and returns the newly created episode summaries for the runtime memory manager.
func Consolidate(hm *consolidator.HistoryManager) ([]EpisodeSummary, error) {
	messages := hm.GetMessages()
	if len(messages) == 0 {
		return nil, fmt.Errorf("no conversation history to consolidate")
	}

	// Filter indices of unstored messages
	var unstoredIndices []int
	for i, msg := range messages {
		if !msg.Stored {
			unstoredIndices = append(unstoredIndices, i)
		}
	}

	if len(unstoredIndices) == 0 {
		return nil, fmt.Errorf("no new messages to consolidate")
	}

	// Determine character limit for consolidation chunking
	maxChars := 3000
	if limitStr := os.Getenv("SYSTEM_CONSOLIDATION_DENSITY"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && limit > 0 {
			maxChars = limit
		}
	}

	// Group unstored indices into chunks
	var chunks [][]int
	var currentChunk []int
	currentLength := 0

	for _, idx := range unstoredIndices {
		msgLen := len(messages[idx].Content)
		if len(currentChunk) > 0 && currentLength+msgLen > maxChars {
			chunks = append(chunks, currentChunk)
			currentChunk = []int{idx}
			currentLength = msgLen
		} else {
			currentChunk = append(currentChunk, idx)
			currentLength += msgLen
		}
	}
	if len(currentChunk) > 0 {
		chunks = append(chunks, currentChunk)
	}

	// Ensure target directory exists
	episodesDir := utils.ResolvePath(filepath.Join("Context", "episodes"))
	if err := os.MkdirAll(episodesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create episodes directory: %w", err)
	}

	// Initialize the summariser agent
	agent := summariser.NewSummariserAgent()

	var newEpisodes []EpisodeSummary

	// ONLY process the first chunk to spread out API calls and avoid rate limits.
	if len(chunks) > 1 {
		chunks = chunks[:1]
	}

	// Process the chunk
	for chunkIdx, indices := range chunks {
		var chunkMsgIDs []string
		var convBuilder strings.Builder

		// Determine peak mindstate in the chunk based on (Negative + Positive Emotion) activation
		peakMindState := "0.90:0.30:0.50:0.70"
		maxActivation := -1.0

		for _, idx := range indices {
			msg := messages[idx]
			chunkMsgIDs = append(chunkMsgIDs, msg.ID)
			convBuilder.WriteString(fmt.Sprintf("%s: %s\n", msg.Author, msg.Content))

			activation := calculateActivationScore(msg.MindState)
			if activation > maxActivation {
				maxActivation = activation
				if msg.MindState != "" {
					peakMindState = msg.MindState
				}
			}
		}

		// Call Summariser agent to get summary JSON
		rawJSON, err := agent.Summarise(context.Background(), convBuilder.String())
		if err != nil {
			return nil, fmt.Errorf("failed to generate summary for chunk %d: %w", chunkIdx, err)
		}

		var llmResp LLMResponse
		if err := json.Unmarshal([]byte(rawJSON), &llmResp); err != nil {
			// Resilient fallback if LLM output is not valid JSON
			llmResp = LLMResponse{
				Summary:    "Failed to parse model summary. raw response: " + rawJSON,
				Conclusion: "Parsed error conclusion.",
			}
		}

		// Save Episode JSON. Use UnixNano to ensure uniqueness across multiple consolidation runs.
		episodeID := fmt.Sprintf("%s_ep_%d", hm.SessionID, time.Now().UnixNano())
		episode := Episode{
			ID:            episodeID,
			Summary:       llmResp.Summary,
			PeakMindState: peakMindState,
			Conclusion:    llmResp.Conclusion,
			MessageIDs:    chunkMsgIDs,
		}

		episodePath := filepath.Join(episodesDir, fmt.Sprintf("%s.json", episodeID))
		episodeData, err := json.MarshalIndent(episode, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to serialize episode %s: %w", episodeID, err)
		}

		if err := os.WriteFile(episodePath, episodeData, 0644); err != nil {
			return nil, fmt.Errorf("failed to write episode file %s: %w", episodePath, err)
		}

		// Generate Embedding
		emb := embedder.NewLocalEmbedder()
		vec, err := emb.Embed(context.Background(), llmResp.Summary)
		if err != nil {
			fmt.Printf("[DEBUG] Failed to generate embedding for %s: %v\n", episodeID, err)
			vec = []float32{} // Fallback empty
		}

		// Append to JSONL Index
		if err := appendToIndexJSONL(episodesDir, peakMindState, vec, episodeID); err != nil {
			return nil, fmt.Errorf("failed to write to index JSONL: %w", err)
		}

		// Mark the messages in HistoryManager as stored on disk
		startIdx := indices[0]
		endIdx := indices[len(indices)-1] + 1
		if err := hm.MarkStored(startIdx, endIdx); err != nil {
			return nil, fmt.Errorf("failed to mark messages as stored for episode %s: %w", episodeID, err)
		}

		// Collect the summary for the runtime memory manager
		newEpisodes = append(newEpisodes, EpisodeSummary{
			ID:            episodeID,
			Summary:       llmResp.Summary,
			PeakMindState: peakMindState,
			Conclusion:    llmResp.Conclusion,
		})
	}

	return newEpisodes, nil
}

// calculateActivationScore calculates the sum of positive + negative emotion from a mindstate string.
func calculateActivationScore(mindState string) float64 {
	if mindState == "" {
		return 0.0
	}
	var ma, ne, pe, ua float64
	n, err := fmt.Sscanf(mindState, "%f:%f:%f:%f", &ma, &ne, &pe, &ua)
	if err != nil || n < 4 {
		return 0.0
	}
	return ne + pe
}

type EpisodeIndex struct {
	EpisodeID     string    `json:"episode_id"`
	PeakMindState string    `json:"peak_mindstate"`
	Embedding     []float32 `json:"embedding"`
}

// appendToIndexJSONL appends the episode embedding to the central index file.
func appendToIndexJSONL(dir, peakMindState string, embedding []float32, episodeID string) error {
	jsonlPath := filepath.Join(dir, "index.jsonl")
	file, err := os.OpenFile(jsonlPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	entry := EpisodeIndex{
		EpisodeID:     episodeID,
		PeakMindState: peakMindState,
		Embedding:     embedding,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	_, err = file.Write(append(data, '\n'))
	return err
}
