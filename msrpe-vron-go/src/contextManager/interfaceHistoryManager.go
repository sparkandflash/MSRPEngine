package contextManager

import (
	"encoding/json"
	"fmt"
	"msrpe-vron-go/src/utils"
	"os"
	"time"
)

// InterfaceHistoryManager handles the rolling flat-file log of all raw I/O (STM).
type InterfaceHistoryManager struct {
	FilePath string
}

type HistoryEntry struct {
	Timestamp string `json:"timestamp"`
	Sender    string `json:"sender"`
	Message   string `json:"message"`
}

// Append writes a single line (message, system event) to the interface history log.
func (ihm *InterfaceHistoryManager) Append(sender string, message string) error {
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
