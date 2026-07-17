package consolidator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Message represents a single chat turn with a role and content.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// STMmanager manages the rolling short term memory of the chat.
type STMmanager struct {
	maxChars int
	messages []Message
}

// NewSTMmanager initializes a new STMmanager with a maximum character limit.
func NewSTMmanager(maxChars int) *STMmanager {
	return &STMmanager{
		maxChars: maxChars,
		messages: []Message{},
	}
}

// Get returns all messages currently stored in short term memory.
func (m *STMmanager) Get() []Message {
	return m.messages
}

// Update appends a message and discards older ones (FIFO) until the total character length is within the maxChars limit.
func (m *STMmanager) Update(role string, content string) {
	m.messages = append(m.messages, Message{Role: role, Content: content})

	// FIFO pruning based on the character length of the contents
	for m.totalChars() > m.maxChars && len(m.messages) > 0 {
		m.messages = m.messages[1:]
	}
}

// totalChars calculates the sum of characters of all messages in short term memory.
func (m *STMmanager) totalChars() int {
	sum := 0
	for _, msg := range m.messages {
		sum += len(msg.Content)
	}
	return sum
}

// HistoryManager manages the persistent log of the full conversation.
type HistoryManager struct {
	SessionID string
	filePath  string
	messages  []Message
}

// NewHistoryManager initializes a new persistent history manager with a timestamp-based SessionID.
func NewHistoryManager() (*HistoryManager, error) {
	sessionID := time.Now().Format("20060102-150405")
	dir := filepath.Join("Context", "conversationHistory")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create conversationHistory directory: %w", err)
	}

	filePath := filepath.Join(dir, fmt.Sprintf("%s.json", sessionID))

	// Initialize the file as an empty JSON array
	if err := os.WriteFile(filePath, []byte("[]"), 0644); err != nil {
		return nil, fmt.Errorf("failed to initialize history file: %w", err)
	}

	return &HistoryManager{
		SessionID: sessionID,
		filePath:  filePath,
		messages:  []Message{},
	}, nil
}

// TODO: Support loading a past conversation by sessionId (SessionID)

// Save appends a new message to the persistent history and writes the full log to disk.
func (h *HistoryManager) Save(role string, content string) error {
	h.messages = append(h.messages, Message{Role: role, Content: content})

	data, err := json.MarshalIndent(h.messages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize history: %w", err)
	}

	if err := os.WriteFile(h.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write history to disk: %w", err)
	}

	return nil
}
