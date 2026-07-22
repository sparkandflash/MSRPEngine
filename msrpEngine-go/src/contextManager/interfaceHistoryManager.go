package contextManager

import (
	"bufio"
	"encoding/json"
	"fmt"
	"msrpengine/src/utils"
	"os"
	"path/filepath"
	"sync"
)

// HistoryManager defines the interface for saving and retrieving conversation history.
type HistoryManager interface {
	SaveHistory(sessionID string, data interface{}) error
	GetHistory(sessionID string) ([]interface{}, error)
}

// JSONLHistoryManager implements HistoryManager using JSON Lines format.
type JSONLHistoryManager struct {
	baseDir string
	mu      sync.RWMutex
}

// NewJSONLHistoryManager initializes a new JSONLHistoryManager.
// It ensures the Context/history directory exists.
func NewJSONLHistoryManager() (*JSONLHistoryManager, error) {
	baseDir := utils.ResolvePath(filepath.Join("Context", "history"))
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create history directory: %w", err)
	}
	return &JSONLHistoryManager{
		baseDir: baseDir,
	}, nil
}

// SaveHistory appends a new entry to the JSONL history file for the given session ID.
func (m *JSONLHistoryManager) SaveHistory(sessionID string, data interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Sanitize sessionID to prevent path traversal attacks
	cleanID := filepath.Base(sessionID)
	filePath := filepath.Join(m.baseDir, fmt.Sprintf("%s.jsonl", cleanID))
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open history file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode data to jsonl: %w", err)
	}

	return nil
}

// GetHistory reads all lines from the JSONL history file and returns them as a slice.
func (m *JSONLHistoryManager) GetHistory(sessionID string) ([]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Sanitize sessionID to prevent path traversal attacks
	cleanID := filepath.Base(sessionID)
	filePath := filepath.Join(m.baseDir, fmt.Sprintf("%s.jsonl", cleanID))
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []interface{}{}, nil // Return empty slice if no history yet
		}
		return nil, fmt.Errorf("failed to open history file: %w", err)
	}
	defer file.Close()

	var history []interface{}
	scanner := bufio.NewScanner(file)
	// Increase maximum line length for the scanner (10MB) to handle large AI contexts
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		var entry interface{}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, fmt.Errorf("failed to unmarshal jsonl line: %w", err)
		}
		history = append(history, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading history file: %w", err)
	}

	return history, nil
}
