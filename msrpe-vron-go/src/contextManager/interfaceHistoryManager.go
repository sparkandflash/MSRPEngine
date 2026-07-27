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

// InterfaceHistoryManager handles the rolling flat-file log of all raw I/O (STM).
type InterfaceHistoryManager struct {
	mu             sync.Mutex
	FilePath       string
	StartupContext string
}

type HistoryEntry struct {
	Timestamp string `json:"timestamp"`
	Sender    string `json:"sender"`
	Message   string `json:"message"`
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
// prepending StartupContext if available, returning the active context block.
func (ihm *InterfaceHistoryManager) ReadRecentContext(maxChars int) string {
	ihm.mu.Lock()
	defer ihm.mu.Unlock()

	f, err := os.Open(ihm.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return ihm.StartupContext
		}
		utils.LogDebug("Failed to open history for read: %v", err)
		return ihm.StartupContext
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

	result := strings.Join(finalBlocks, "")
	if ihm.StartupContext != "" {
		if result != "" {
			result = ihm.StartupContext + "\n\n" + result
		} else {
			result = ihm.StartupContext
		}
	}

	return result
}
