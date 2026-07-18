package reflector

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"terminal-app/src/embedder"
	"terminal-app/src/responder"
	"terminal-app/src/utils"
)

// EpisodeIndex mirrors the structure stored in index.jsonl
type EpisodeIndex struct {
	EpisodeID     string    `json:"episode_id"`
	PeakMindState string    `json:"peak_mindstate"`
	Embedding     []float32 `json:"embedding"`
}

// match is a helper struct for sorting similarities
type match struct {
	episodeID  string
	similarity float64
	attn       float64
}

// Reflect finds episodes in index.jsonl with a higher attention score than currentMindState,
// and which have the highest cosine similarity to the active episode summaries.
func Reflect(currentMindState string, activeEpisodes []responder.EpisodeSummary) ([]string, error) {
	currentAttn, err := parseAttentionScore(currentMindState)
	if err != nil {
		return nil, fmt.Errorf("failed to parse current mindstate: %w", err)
	}

	// 1. Build query text from active episodes
	var queryBuilder strings.Builder
	for _, ep := range activeEpisodes {
		queryBuilder.WriteString(ep.Summary + " ")
	}
	queryText := strings.TrimSpace(queryBuilder.String())

	// 2. Generate Query Embedding
	emb := embedder.NewLocalEmbedder()
	var queryVec []float32
	if queryText != "" {
		vec, err := emb.Embed(context.Background(), queryText)
		if err == nil {
			queryVec = vec
		} else {
			fmt.Printf("[DEBUG] Failed to embed query for reflection: %v\n", err)
		}
	}

	// 3. Read index.jsonl
	indexPath := utils.ResolvePath(filepath.Join("Context", "episodes", "index.jsonl"))
	file, err := os.Open(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No episodes yet
		}
		return nil, fmt.Errorf("failed to open index.jsonl: %w", err)
	}
	defer file.Close()

	var matches []match
	var highestAttnEpisode string
	var highestAttnScore float64

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var row EpisodeIndex
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue // Skip corrupted lines
		}

		rowAttn, err := parseAttentionScore(row.PeakMindState)
		if err != nil {
			continue
		}

		// Keep track of the highest attention episode to seed memory if completely empty
		if rowAttn > highestAttnScore {
			highestAttnScore = rowAttn
			highestAttnEpisode = row.EpisodeID
		}

		// Filter 1: Episode attention must be greater than current attention
		if rowAttn <= currentAttn {
			continue
		}

		// Filter 2: Calculate Semantic Similarity
		sim := 0.0
		if len(queryVec) > 0 && len(row.Embedding) > 0 {
			sim = embedder.CosineSimilarity(queryVec, row.Embedding)
		} else {
			// If no embeddings available, use a nominal similarity based on attention
			sim = rowAttn
		}

		matches = append(matches, match{
			episodeID:  row.EpisodeID,
			similarity: sim,
			attn:       rowAttn,
		})
	}

	// 4. Sort and select top matches
	if len(matches) > 0 {
		// Sort by similarity descending
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].similarity > matches[j].similarity
		})

		var topIDs []string
		for i := 0; i < len(matches) && i < 3; i++ { // Return top 3
			topIDs = append(topIDs, matches[i].episodeID)
		}
		return topIDs, nil
	}

	// Fallback: If no matches and active memory is empty, just return the highest attention episode to seed it.
	if highestAttnEpisode != "" && highestAttnScore > currentAttn {
		return []string{highestAttnEpisode}, nil
	}

	return nil, nil
}

// parseAttentionScore extracts MA (Model Attention) and UA (User Attention) from a mindstate string
// formatted as "MA:NE:PE:UA" and returns their sum.
func parseAttentionScore(mindState string) (float64, error) {
	parts := strings.Split(mindState, ":")
	if len(parts) != 4 {
		return 0, fmt.Errorf("invalid mindstate format")
	}

	ma, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, err
	}
	ua, err := strconv.ParseFloat(parts[3], 64)
	if err != nil {
		return 0, err
	}
	return ma + ua, nil
}
