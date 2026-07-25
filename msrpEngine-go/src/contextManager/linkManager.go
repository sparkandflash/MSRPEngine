package contextManager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"msrpengine/src/utils"
)

// EpisodeLink represents a conceptual bridge between two episodes.
type EpisodeLink struct {
	ID          string `json:"id"`
	SourceEpID  string `json:"source_ep_id"`
	TargetEpID  string `json:"target_ep_id"`
	Summary     string `json:"summary"`
	AccessCount int    `json:"access_count"`
	IsNegative  bool   `json:"is_negative"`
}

type LinkManager struct {
	mu       sync.RWMutex
	links    map[string]EpisodeLink
	linksDir string
}

var (
	globalLinkManager *LinkManager
	linkManagerOnce   sync.Once
)

func GetLinkManager() *LinkManager {
	linkManagerOnce.Do(func() {
		globalLinkManager = NewLinkManager()
	})
	return globalLinkManager
}

func NewLinkManager() *LinkManager {
	dir := utils.ResolvePath(filepath.Join("Context", "links"))
	_ = os.MkdirAll(dir, 0755)

	lm := &LinkManager{
		links:    make(map[string]EpisodeLink),
		linksDir: dir,
	}
	lm.LoadAll()
	return lm
}

func (lm *LinkManager) LoadAll() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	entries, err := os.ReadDir(lm.linksDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(lm.linksDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var link EpisodeLink
		if err := json.Unmarshal(data, &link); err == nil {
			lm.links[link.ID] = link
		}
	}
}

func (lm *LinkManager) SaveLink(link EpisodeLink) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.links[link.ID] = link

	data, err := json.MarshalIndent(link, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(lm.linksDir, link.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// saveToDisk writes a link to disk. Caller must NOT hold the mutex.
func (lm *LinkManager) saveToDisk(link EpisodeLink) error {
	data, err := json.MarshalIndent(link, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(lm.linksDir, link.ID+".json")
	return os.WriteFile(path, data, 0644)
}

func (lm *LinkManager) GetLinksForEpisode(epID string) []EpisodeLink {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	var result []EpisodeLink
	for _, link := range lm.links {
		if link.SourceEpID == epID || link.TargetEpID == epID {
			result = append(result, link)
		}
	}
	return result
}

func (lm *LinkManager) MarkUseful(linkID string) error {
	lm.mu.Lock()
	link, exists := lm.links[linkID]
	if !exists {
		lm.mu.Unlock()
		return fmt.Errorf("link not found")
	}
	link.AccessCount++
	lm.links[linkID] = link
	lm.mu.Unlock()
	return lm.saveToDisk(link)
}

func (lm *LinkManager) MarkNegative(linkID string) error {
	lm.mu.Lock()
	link, exists := lm.links[linkID]
	if !exists {
		lm.mu.Unlock()
		return fmt.Errorf("link not found")
	}
	link.IsNegative = true
	lm.links[linkID] = link
	lm.mu.Unlock()
	return lm.saveToDisk(link)
}
