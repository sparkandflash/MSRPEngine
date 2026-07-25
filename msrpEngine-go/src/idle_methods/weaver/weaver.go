package weaver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"msrpengine/src/contextManager"
	"msrpengine/src/idle_methods/episode_memory"
	"msrpengine/src/prompts"
	"msrpengine/src/agents/summariser"
	"github.com/google/uuid"
)

// Weave runs in the background, picks a recent episode, finds a semantically similar episode,
// and creates an EpisodeLink summarizing their relationship.
func Weave(episodeMgr *episode_memory.EpisodeMemoryManager, linkMgr *contextManager.LinkManager) error {
	activeEps := episodeMgr.GetActive()
	if len(activeEps) == 0 {
		return fmt.Errorf("no active episodes to weave")
	}

	// 1. Pick the most recently accessed episode (usually the last active one)
	targetEp := activeEps[len(activeEps)-1]

	// 2. Find a semantically similar historical episode using chromem
	idxMgr, err := contextManager.NewChromemIndexManager()
	if err != nil {
		return fmt.Errorf("failed to init index manager: %w", err)
	}

	query := strings.TrimSpace(strings.Join(targetEp.Facts, " "))
	if query == "" {
		return fmt.Errorf("target episode has no facts to weave from")
	}

	// Fetch a larger pool of results because the target episode's own facts will naturally
	// be the most similar matches and consume the top slots. We need to guarantee we pull
	// past those self-matches to find genuine historical correlations.
	searchLimit := len(targetEp.Facts) + 10 //remove this when we make facts into nodes.
	results, err := idxMgr.SearchContext("facts", query, searchLimit)
	if err != nil {
		return fmt.Errorf("failed to search context: %w", err)
	}

	var matchID string
	var matchFact string
	for _, res := range results {
		// Chromem stores facts as "{episodeID}_fact_{N}" — extract the base episode ID
		baseID := res.ID
		if idx := strings.LastIndex(res.ID, "_fact_"); idx != -1 {
			baseID = res.ID[:idx]
		}
		if baseID != targetEp.ID {
			matchID = baseID
			matchFact = res.Document
			break
		}
	}

	if matchID == "" {
		return fmt.Errorf("no similar historical episode found")
	}

	// 3. Prevent duplicate POSITIVE links. A negative link between the same pair
	// is not a blocker — the Weaver may still form a new positive link later
	// after the relationship has evolved in a different context.
	existingLinks := linkMgr.GetLinksForEpisode(targetEp.ID)
	for _, l := range existingLinks {
		if !l.IsNegative && (l.SourceEpID == matchID || l.TargetEpID == matchID) {
			return fmt.Errorf("positive link already exists between %s and %s", targetEp.ID, matchID)
		}
	}

	// 4. Summarize the connection using LLM
	prompt := prompts.GetWeaverPrompt()
	transcript := fmt.Sprintf("Current Episode (%s):\n%s\n\nHistorical Episode (%s):\n%s", targetEp.ID, query, matchID, matchFact)

	summariserAgent := summariser.NewSummariserAgent()
	respStr, err := summariserAgent.SummariseWithPrompt(context.Background(), transcript, prompt)
	if err != nil {
		return fmt.Errorf("failed to generate weave summary: %w", err)
	}

	var result struct {
		ConnectionSummary string `json:"connection_summary"`
	}
	if err := json.Unmarshal([]byte(respStr), &result); err != nil {
		return fmt.Errorf("failed to parse weaver json: %w", err)
	}
	
	if result.ConnectionSummary == "" {
		return fmt.Errorf("empty connection summary generated")
	}

	// 5. Create and save the Link
	linkID := "link_" + uuid.New().String()
	newLink := contextManager.EpisodeLink{
		ID:          linkID,
		SourceEpID:  targetEp.ID,
		TargetEpID:  matchID,
		Summary:     result.ConnectionSummary,
		AccessCount: 0,
		IsNegative:  false,
	}

	err = linkMgr.SaveLink(newLink)
	if err != nil {
		return fmt.Errorf("failed to save new link: %w", err)
	}

	return nil
}
