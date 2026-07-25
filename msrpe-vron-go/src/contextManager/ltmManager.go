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
