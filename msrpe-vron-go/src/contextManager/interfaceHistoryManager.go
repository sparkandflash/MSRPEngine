package contextManager

import (
	"bufio"
	"encoding/json"
	"fmt"
	"msrpe-vron-go/src/utils"
	"os"
	"strings"
	"sync"
	"time"
)

// InterfaceHistoryManager handles the rolling flat-file log of all raw I/O (STM)
// and maintains the Short-Term Fact Store (STFS) holding up to 10 active in-memory facts.
type InterfaceHistoryManager struct {
	mu                 sync.Mutex
	FilePath           string
	StartupContext     string
	ShortTermFactStore []string // Active in-memory fact buffer (max 10 facts)
}

type HistoryEntry struct {
	Timestamp string `json:"timestamp"`
	Sender    string `json:"sender"`
	Message   string `json:"message"`
}

// AddShortTermFact adds a retrieved or newly saved fact into the in-memory Short-Term Fact Store (STFS),
// maintaining a maximum capacity of 10 facts (LRU/FIFO replacement).
func (ihm *InterfaceHistoryManager) AddShortTermFact(fact string) {
	ihm.mu.Lock()
	defer ihm.mu.Unlock()

	fact = strings.TrimSpace(fact)
	if fact == "" {
		return
	}

	// Deduplicate within the short-term fact buffer (case-insensitive)
	for i, existing := range ihm.ShortTermFactStore {
		if strings.EqualFold(existing, fact) {
			// Move to end (most recently accessed)
			ihm.ShortTermFactStore = append(ihm.ShortTermFactStore[:i], ihm.ShortTermFactStore[i+1:]...)
			ihm.ShortTermFactStore = append(ihm.ShortTermFactStore, fact)
			return
		}
	}

	// Evict oldest if capacity (10 facts) reached
	if len(ihm.ShortTermFactStore) >= 10 {
		ihm.ShortTermFactStore = ihm.ShortTermFactStore[1:]
	}
	ihm.ShortTermFactStore = append(ihm.ShortTermFactStore, fact)
	utils.LogInfo("[STFS] Short-Term Fact Store updated (%d/10 facts): %q", len(ihm.ShortTermFactStore), fact)
}

// GetShortTermFactStoreFormatted returns the active STFS formatted for prompt injection.
func (ihm *InterfaceHistoryManager) GetShortTermFactStoreFormatted() string {
	if len(ihm.ShortTermFactStore) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("--- SHORT-TERM FACT STORE (ACTIVE IN-MEMORY FACTS) ---\n")
	for i, fact := range ihm.ShortTermFactStore {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, fact))
	}
	return strings.TrimSpace(sb.String())
}

// Append writes a single line (message, system event) to the interface history log.
func (ihm *InterfaceHistoryManager) Append(sender string, message string) error {
	ihm.mu.Lock()
	defer ihm.mu.Unlock()

	entry := HistoryEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Sender:    sender,
		Message:   message,
	}

	b, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal history entry: %v", err)
	}
	logEntry := string(b) + "\n"

	f, err := os.OpenFile(ihm.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open history file: %v", err)
	}
	defer f.Close()

	n, err := f.WriteString(logEntry)
	if err != nil {
		return fmt.Errorf("failed to write to history file: %v", err)
	}

	utils.LogDebug("Appended %d bytes to %s", n, ihm.FilePath)
	return nil
}

// ReadRecentContext reads the JSONL history file from bottom to top,
// prepending StartupContext and the Short-Term Fact Store (STFS), returning the active context block.
func (ihm *InterfaceHistoryManager) ReadRecentContext(maxChars int) string {
	ihm.mu.Lock()
	defer ihm.mu.Unlock()

	f, err := os.Open(ihm.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return ihm.formatContextBlock("")
		}
		utils.LogDebug("Failed to open history for read: %v", err)
		return ihm.formatContextBlock("")
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	var finalBlocks []string
	totalChars := 0

	for i := len(lines) - 1; i >= 0; i-- {
		var entry HistoryEntry
		if err := json.Unmarshal([]byte(lines[i]), &entry); err != nil {
			continue
		}

		formatted := fmt.Sprintf("[%s]: %s\n", entry.Sender, entry.Message)
		if totalChars+len(formatted) > maxChars {
			break
		}
		finalBlocks = append([]string{formatted}, finalBlocks...)
		totalChars += len(formatted)
	}

	rawHistory := strings.Join(finalBlocks, "")
	return ihm.formatContextBlock(rawHistory)
}

// formatContextBlock combines StartupContext, STFS, and rawHistory.
func (ihm *InterfaceHistoryManager) formatContextBlock(rawHistory string) string {
	var parts []string

	if ihm.StartupContext != "" {
		parts = append(parts, ihm.StartupContext)
	}

	stfs := ihm.GetShortTermFactStoreFormatted()
	if stfs != "" {
		parts = append(parts, stfs)
	}

	if rawHistory != "" {
		parts = append(parts, rawHistory)
	}

	return strings.Join(parts, "\n\n")
}
