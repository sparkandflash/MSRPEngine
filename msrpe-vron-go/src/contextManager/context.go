package contextManager

import (
	"encoding/json"
	"fmt"
	"msrpe-vron-go/src/utils"
	"os"
	"path/filepath"
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

// SaveSpecialEpisode packages explicit facts into character-chunked fact_<timestamp> files,
// saves them to disk with timestamp & MA:UA:SE:OX:CO mind state, and indexes all facts into the vector DB.
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

	// Fact episode type: fact_<timestamp>
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

	for _, fact := range facts {
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
		// Only consider main consolidated episodes starting with "ep_", NOT fact_
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
