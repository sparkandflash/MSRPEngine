package contextManager

import (
	"encoding/json"
	"fmt"
	"msrpe-vron-go/src/utils"
	"os"
	"path/filepath"
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
	Type      string `json:"type"`      // e.g. "fact", "summary", "interaction"
	Content   string `json:"content"`   // The semantic content to be embedded
	Timestamp int64  `json:"timestamp"` // Unix timestamp
	Weight    int    `json:"weight"`    // Cognitive weight/confidence
}

// ContextManager coordinates interface history, episodes, and the vector DB.
type ContextManager struct {
	IndexManager   *ContextIndexManager
	HistoryManager *InterfaceHistoryManager
}

// NewContextManager initializes directories and sub-managers.
func NewContextManager() (*ContextManager, error) {
	utils.LogInfo("Initializing Context Manager...")

	// 1. Ensure directory structures exist
	dirs := []string{ContextDir, EpisodesDir, ChromemDB}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %v", dir, err)
		}
	}

	// 2. Initialize Sub-Managers
	idxManager, err := NewContextIndexManager()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize IndexManager: %v", err)
	}

	histManager := &InterfaceHistoryManager{
		FilePath: filepath.Join(ContextDir, "interface_history.json"),
	}

	return &ContextManager{
		IndexManager:   idxManager,
		HistoryManager: histManager,
	}, nil
}

// SaveEpisode writes the episode to disk and embeds it into the vector database.
func (cm *ContextManager) SaveEpisode(content string, epType string, weight int) error {
	ep := Episode{
		ID:        uuid.New().String(),
		Type:      epType,
		Content:   content,
		Timestamp: time.Now().Unix(),
		Weight:    weight,
	}

	// 1. Save JSON to disk
	b, err := json.MarshalIndent(ep, "", "  ")
	if err != nil {
		return err
	}
	filePath := filepath.Join(EpisodesDir, fmt.Sprintf("ep_%s.json", ep.ID))
	if err := os.WriteFile(filePath, b, 0644); err != nil {
		return err
	}

	utils.LogDebug("Saved Episode to Disk: %s", filePath)

	// 2. Embed into Vector DB
	return cm.IndexManager.IndexEpisode(ep)
}
