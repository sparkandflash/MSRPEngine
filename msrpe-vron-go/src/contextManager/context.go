package contextManager

import (
	"encoding/json"
	"fmt"
	"msrpe-vron-go/src/utils"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Base paths for context storage
const (
	ContextDir  = "Context"
	EpisodesDir = "Context/episodes"
	ChromemDB   = "Context/chromem_db"
)

// Episode represents a single node of factual memory or interaction.
type Episode struct {
	ID        string `json:"id"`
	Type      string `json:"type"`      // e.g. "summary", "fact_1722000000"
	Content   string `json:"content"`   // The semantic content to be embedded
	Timestamp int64  `json:"timestamp"` // Unix timestamp
	Weight    int    `json:"weight"`    // Cognitive weight/confidence
}

// SpecialEpisode represents a structured memory node containing multiple facts, mind scores, and timestamp.
type SpecialEpisode struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`      // "fact_<timestamp>"
	Facts     []string `json:"facts"`     // List of facts stored in this episode
	MindState string   `json:"mindstate"` // MA:UA:SE:OX:CO snapshot e.g. "0.90:0.30:0.00:0.50:0.10"
	Timestamp int64    `json:"timestamp"` // Unix timestamp
	Weight    int      `json:"weight"`
}

// ContextManager coordinates interface history, episodes, and the vector DB.
type ContextManager struct {
	IndexManager   *ContextIndexManager
	HistoryManager *InterfaceHistoryManager
}

// NewContextManager initializes directories and sub-managers.
func NewContextManager() (*ContextManager, error) {
	utils.LogInfo("Initializing Context Manager...")

	dirs := []string{ContextDir, EpisodesDir, ChromemDB}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %v", dir, err)
		}
	}

	idxManager, err := NewContextIndexManager()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize IndexManager: %v", err)
	}

	histManager := &InterfaceHistoryManager{
		FilePath: filepath.Join(ContextDir, "interface_history.jsonl"),
	}

	return &ContextManager{
		IndexManager:   idxManager,
		HistoryManager: histManager,
	}, nil
}

// IsDuplicateFact checks if a given fact string is already present in the active STFS,
// stored fact episodes on disk, or vector database.
func (cm *ContextManager) IsDuplicateFact(fact string) bool {
	cleanFact := strings.ToLower(strings.TrimSpace(fact))
	if cleanFact == "" {
		return true
	}

	// 1. Check Short-Term Fact Store in memory
	if cm.HistoryManager != nil {
		cm.HistoryManager.mu.Lock()
		for _, stFact := range cm.HistoryManager.ShortTermFactStore {
			stClean := strings.ToLower(strings.TrimSpace(stFact))
			if stClean == cleanFact || strings.Contains(stClean, cleanFact) || strings.Contains(cleanFact, stClean) {
				cm.HistoryManager.mu.Unlock()
				return true
			}
		}
		cm.HistoryManager.mu.Unlock()
	}

	// 2. Check stored fact episodes on disk
	files, err := os.ReadDir(EpisodesDir)
	if err == nil {
		for _, f := range files {
			if f.IsDir() || !strings.HasPrefix(f.Name(), "fact_") || !strings.HasSuffix(f.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(EpisodesDir, f.Name()))
			if err != nil {
				continue
			}
			var spEp SpecialEpisode
			if err := json.Unmarshal(data, &spEp); err == nil {
				for _, storedFact := range spEp.Facts {
					storedClean := strings.ToLower(strings.TrimSpace(storedFact))
					if storedClean == cleanFact || strings.Contains(storedClean, cleanFact) || strings.Contains(cleanFact, storedClean) {
						return true
					}
				}
			}
		}
	}

	// 3. Query vector DB if available for semantic similarity
	if cm.IndexManager != nil {
		col := cm.IndexManager.Client.GetCollection("episodes", nil)
		if col != nil && col.Count() > 0 {
			episodes, err := cm.IndexManager.QueryEpisodes(fact, 1)
			if err == nil && len(episodes) > 0 {
				topClean := strings.ToLower(strings.TrimSpace(episodes[0].Content))
				if topClean == cleanFact || strings.Contains(topClean, cleanFact) || strings.Contains(cleanFact, topClean) {
					return true
				}
			}
		}
	}

	return false
}

// GetAllActiveFacts collects all stored facts across all fact_*.json files, ordered chronologically by timestamp.
func (cm *ContextManager) GetAllActiveFacts() []string {
	files, err := os.ReadDir(EpisodesDir)
	if err != nil {
		return nil
	}

	type timeFact struct {
		timestamp int64
		fact      string
	}
	var timeFacts []timeFact

	for _, f := range files {
		if f.IsDir() || !strings.HasPrefix(f.Name(), "fact_") || !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(EpisodesDir, f.Name()))
		if err != nil {
			continue
		}
		var spEp SpecialEpisode
		if err := json.Unmarshal(data, &spEp); err == nil {
			for _, fact := range spEp.Facts {
				timeFacts = append(timeFacts, timeFact{
					timestamp: spEp.Timestamp,
					fact:      fact,
				})
			}
		}
	}

	sort.Slice(timeFacts, func(i, j int) bool {
		return timeFacts[i].timestamp < timeFacts[j].timestamp
	})

	var facts []string
	for _, tf := range timeFacts {
		facts = append(facts, tf.fact)
	}
	return facts
}

// GetAllStoredFactsFormatted returns all active facts formatted for prompt injection during consolidation.
func (cm *ContextManager) GetAllStoredFactsFormatted() string {
	facts := cm.GetAllActiveFacts()
	if len(facts) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("--- ACTIVE FACT STORE (FACT EPISODES) ---\n")
	for i, fact := range facts {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, fact))
	}
	return strings.TrimSpace(sb.String())
}

// SaveEpisode writes a main consolidated episode (ep_*.json) to disk and embeds it into the vector DB.
func (cm *ContextManager) SaveEpisode(content string, epType string, weight int) error {
	epID := uuid.New().String()
	if epType == "" {
		epType = "summary"
	}
	ep := Episode{
		ID:        epID,
		Type:      epType,
		Content:   content,
		Timestamp: time.Now().Unix(),
		Weight:    weight,
	}

	b, err := json.MarshalIndent(ep, "", "  ")
	if err != nil {
		return err
	}
	filePath := filepath.Join(EpisodesDir, fmt.Sprintf("ep_%s.json", ep.ID))
	if err := os.WriteFile(filePath, b, 0644); err != nil {
		return err
	}

	utils.LogDebug("Saved Main Consolidated Episode to Disk: %s", filePath)

	return cm.IndexManager.IndexEpisode(ep)
}

// SaveSpecialEpisode packages unique non-duplicate facts into character-chunked fact_<timestamp> files,
// saves them to disk with timestamp & mind state, pushes them to STFS, and indexes them in the vector DB.
func (cm *ContextManager) SaveSpecialEpisode(facts []string, epType string, mindState string, weight int, maxCharsPerEpisode int) error {
	if len(facts) == 0 {
		return nil
	}

	if maxCharsPerEpisode <= 0 {
		maxCharsPerEpisode = 2000
	}

	mindStateStr := mindState
	if mindStateStr == "" {
		mindStateStr = "0.90:0.30:0.00:0.50:0.10"
	}
	now := time.Now().Unix()

	// Filter candidate facts for duplicates
	var uniqueFacts []string
	for _, fact := range facts {
		fact = strings.TrimSpace(fact)
		if fact == "" {
			continue
		}
		if cm.IsDuplicateFact(fact) {
			utils.LogInfo("[ContextManager] Duplicate fact skipped: %q", fact)
			continue
		}
		uniqueFacts = append(uniqueFacts, fact)

		// Push to Short-Term Fact Store (STFS)
		if cm.HistoryManager != nil {
			cm.HistoryManager.AddShortTermFact(fact)
		}
	}

	if len(uniqueFacts) == 0 {
		utils.LogInfo("[ContextManager] All facts in batch were duplicates. No new fact episode created.")
		return nil
	}

	factType := fmt.Sprintf("fact_%d", now)
	var currentChunk []string
	currentLen := 0

	saveChunk := func(chunk []string) error {
		if len(chunk) == 0 {
			return nil
		}
		epID := uuid.New().String()
		spEp := SpecialEpisode{
			ID:        epID,
			Type:      factType,
			Facts:     chunk,
			MindState: mindStateStr,
			Timestamp: now,
			Weight:    weight,
		}

		b, err := json.MarshalIndent(spEp, "", "  ")
		if err != nil {
			return err
		}
		filePath := filepath.Join(EpisodesDir, fmt.Sprintf("fact_%d_%s.json", now, epID))
		if err := os.WriteFile(filePath, b, 0644); err != nil {
			return err
		}

		combinedContent := strings.Join(chunk, " | ")
		ep := Episode{
			ID:        spEp.ID,
			Type:      factType,
			Content:   combinedContent,
			Timestamp: now,
			Weight:    weight,
		}
		utils.LogInfo("[ContextManager] Saved Fact Episode %s (%d facts, Type: %s, MindState: %s)", spEp.ID, len(chunk), factType, mindStateStr)
		return cm.IndexManager.IndexEpisode(ep)
	}

	for _, fact := range uniqueFacts {
		factLen := len(fact)
		if len(currentChunk) > 0 && currentLen+factLen > maxCharsPerEpisode {
			if err := saveChunk(currentChunk); err != nil {
				return err
			}
			currentChunk = []string{fact}
			currentLen = factLen
		} else {
			currentChunk = append(currentChunk, fact)
			currentLen += factLen
		}
	}

	if len(currentChunk) > 0 {
		return saveChunk(currentChunk)
	}

	return nil
}

// GetLatestConsolidatedEpisode reads disk for the single most recent main consolidated episode (ep_*.json).
// Returns empty string if no main consolidated episode is saved. Fact episodes (fact_*) are ignored.
func (cm *ContextManager) GetLatestConsolidatedEpisode() (string, error) {
	files, err := os.ReadDir(EpisodesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	var latestPath string
	var latestTime time.Time

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		if strings.HasPrefix(name, "ep_") && strings.HasSuffix(name, ".json") {
			info, err := f.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(latestTime) {
				latestTime = info.ModTime()
				latestPath = filepath.Join(EpisodesDir, name)
			}
		}
	}

	if latestPath == "" {
		return "", nil
	}

	data, err := os.ReadFile(latestPath)
	if err != nil {
		return "", err
	}

	var ep Episode
	if err := json.Unmarshal(data, &ep); err != nil {
		return "", err
	}

	formatted := fmt.Sprintf("[Restored Memory from Previous Consolidation Session]\n%s", ep.Content)
	utils.LogInfo("[ContextManager] Found latest consolidated episode: %s", filepath.Base(latestPath))
	return formatted, nil
}
