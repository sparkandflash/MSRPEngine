package contextManager

import (
	"fmt"
	"msrpe-vron-go/src/utils"
	"os"
	"time"
)

// InterfaceHistoryManager handles the rolling flat-file log of all raw I/O (STM).
type InterfaceHistoryManager struct {
	FilePath string
}

// Append writes a single line (message, system event) to the interface history log.
func (ihm *InterfaceHistoryManager) Append(sender string, message string) error {
	timestamp := time.Now().Format(time.RFC3339)
	logEntry := fmt.Sprintf("[%s] %s: %s\n", timestamp, sender, message)

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
