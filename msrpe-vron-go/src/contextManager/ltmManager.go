package contextManager

import (
	"fmt"
	"msrpe-vron-go/src/utils"
	"strings"
)

// SearchLTM queries the vector DB for the most semantically relevant episodes
// to the given query string. Returns formatted text ready for injection into VRonContext.LTM.
func (cm *ContextManager) SearchLTM(query string, maxResults int) (string, error) {
	if query == "" {
		return "", nil
	}

	col := cm.IndexManager.Client.GetCollection("episodes", nil)
	if col == nil || col.Count() == 0 {
		return "", nil
	}

	episodes, err := cm.IndexManager.QueryEpisodes(query, maxResults)
	if err != nil {
		utils.LogDebug("LTM search failed: %v", err)
		return "", err
	}

	if len(episodes) == 0 {
		utils.LogDebug("LTM search returned 0 results for query: %s", query)
		return "", nil
	}

	// Format results as readable text for the LLM
	var sb strings.Builder
	for i, ep := range episodes {
		sb.WriteString(fmt.Sprintf("[Memory %d | Type: %s | Weight: %d]\n%s\n\n",
			i+1, ep.Type, ep.Weight, ep.Content))
	}

	result := strings.TrimSpace(sb.String())
	utils.LogDebug("LTM Search | Query: %q | Results: %d", query, len(episodes))
	return result, nil
}

// SearchLTMConsolidatedOnly queries the vector DB excluding fact_ / special_fact episodes.
// Used by ContextSwap to only refresh with main consolidated summary episodes.
func (cm *ContextManager) SearchLTMConsolidatedOnly(query string, maxResults int) (string, error) {
	if query == "" {
		return "", nil
	}

	col := cm.IndexManager.Client.GetCollection("episodes", nil)
	if col == nil || col.Count() == 0 {
		return "", nil
	}

	episodes, err := cm.IndexManager.QueryEpisodes(query, maxResults)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	count := 0
	for _, ep := range episodes {
		// Filter out fact_ and special_fact episodes
		if strings.HasPrefix(ep.Type, "fact_") || ep.Type == "special_fact" {
			continue
		}
		count++
		sb.WriteString(fmt.Sprintf("[Consolidated Memory %d | Type: %s]\n%s\n\n", count, ep.Type, ep.Content))
	}

	if count == 0 {
		return "", nil
	}

	return strings.TrimSpace(sb.String()), nil
}
