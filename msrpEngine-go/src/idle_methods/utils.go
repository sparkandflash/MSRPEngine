package idle_methods

import (
	"math"
	"strconv"
	"strings"

	"msrpengine/src/contextManager"
	"msrpengine/src/idle_methods/episode_memory"
)

// CalculateMindStateDistance computes the Euclidean distance between two mind states.
func CalculateMindStateDistance(state1, state2 string) float64 {
	parts1 := strings.Split(state1, ":")
	parts2 := strings.Split(state2, ":")

	if len(parts1) != 5 || len(parts2) != 5 {
		return 0
	}

	var dist float64
	for i := 0; i < 5; i++ {
		val1, _ := strconv.ParseFloat(parts1[i], 64)
		val2, _ := strconv.ParseFloat(parts2[i], 64)
		dist += math.Pow(val1-val2, 2)
	}
	return math.Sqrt(dist)
}

// GetEpisodesByCharCount iterates over a prioritized list of episodes and returns as many as fit in charLimit.
// It uses energy drain rate to dynamically reduce the base char limit (higher rate -> less context).
// drainRate is in the engine's native range of ~10 (calm) to ~30 (panicked).
// We normalize this to a 0.0–1.0 scale and apply a gentle 50% max reduction at peak drain.
func GetEpisodesByCharCount(episodeList []episode_memory.EpisodeSummary, baseCharLimit int, drainRate float64) []episode_memory.EpisodeSummary {
	const minDrain, maxDrain = 10.0, 30.0
	normalized := math.Max(0, math.Min(1.0, (drainRate-minDrain)/(maxDrain-minDrain)))
	// At normalized=0 (calm): limit = 100% of base. At normalized=1 (panic): limit = 50% of base.
	adjustedLimit := int(float64(baseCharLimit) * (1.0 - 0.5*normalized))
	if adjustedLimit <= 0 {
		return nil
	}

	var result []episode_memory.EpisodeSummary
	currentChars := 0

	for _, ep := range episodeList {
		epChars := 0
		for _, fact := range ep.Facts {
			epChars += len(fact)
		}

		if currentChars+epChars <= adjustedLimit {
			result = append(result, ep)
			currentChars += epChars
		} else {
			// Stop: all remaining episodes are pre-sorted by relevance priority,
			// so once one doesn't fit the limit is hit.
			break
		}
	}

	return result
}

// GetInterfaceEventsByCharCount keeps the most recent events (trimming older ones) up to charLimit.
// drainRate is in the engine's native range of ~10 (calm) to ~30 (panicked).
func GetInterfaceEventsByCharCount(eventList []contextManager.InterfaceEvent, baseCharLimit int, drainRate float64) []contextManager.InterfaceEvent {
	const minDrain, maxDrain = 10.0, 30.0
	normalized := math.Max(0, math.Min(1.0, (drainRate-minDrain)/(maxDrain-minDrain)))
	// At normalized=0 (calm): limit = 100% of base. At normalized=1 (panic): limit = 50% of base.
	adjustedLimit := int(float64(baseCharLimit) * (1.0 - 0.5*normalized))
	if adjustedLimit <= 0 {
		return nil
	}

	var result []contextManager.InterfaceEvent
	currentChars := 0

	// eventList is ordered oldest to newest. We want to keep newest, so we iterate backwards.
	for i := len(eventList) - 1; i >= 0; i-- {
		ev := eventList[i]
		evChars := len(ev.Content) + len(ev.Author)
		if currentChars+evChars <= adjustedLimit {
			// Prepend since we are iterating backwards
			result = append([]contextManager.InterfaceEvent{ev}, result...)
			currentChars += evChars
		} else {
			break
		}
	}

	return result
}
