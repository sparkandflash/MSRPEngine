package consolidator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Message represents a single chat turn with a role, content, mindstate, and stored flag.
type Message struct {
	Author    string `json:"author"`
	Content   string `json:"content"`
	MindState string `json:"mindstate,omitempty"`
	Stored    bool   `json:"stored"`
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

// GetNoFlags returns all messages in STM with only Author and Content populated.
// MindState and Stored flags are omitted — this is the clean view sent to the responder LLM.
func (m *STMmanager) GetNoFlags() []Message {
	clean := make([]Message, len(m.messages))
	for i, msg := range m.messages {
		clean[i] = Message{Author: msg.Author, Content: msg.Content}
	}
	return clean
}

// Update appends a message and discards older ones (FIFO) until the total character length is within the maxChars limit.
func (m *STMmanager) Update(role string, content string) {
	m.messages = append(m.messages, Message{Author: role, Content: content})

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

// NewHistoryManager initializes a persistent history manager for a given SessionID.
// If the session file already exists, it loads the historical messages into memory.
func NewHistoryManager(sessionID string) (*HistoryManager, error) {
	if sessionID == "" {
		sessionID = time.Now().Format("20060102-150405")
	}

	dir := filepath.Join("Context", "conversationHistory")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create conversationHistory directory: %w", err)
	}

	filePath := filepath.Join(dir, fmt.Sprintf("%s.json", sessionID))

	messages := []Message{}
	
	// If the file exists, attempt to load it
	if _, err := os.Stat(filePath); err == nil {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read existing history file: %w", err)
		}
		if len(data) > 0 {
			if err := json.Unmarshal(data, &messages); err != nil {
				return nil, fmt.Errorf("failed to parse existing history file: %w", err)
			}
		}
	} else {
		// Initialize the file as an empty JSON array
		if err := os.WriteFile(filePath, []byte("[]"), 0644); err != nil {
			return nil, fmt.Errorf("failed to initialize history file: %w", err)
		}
	}

	return &HistoryManager{
		SessionID: sessionID,
		filePath:  filePath,
		messages:  messages,
	}, nil
}

// Save appends a new message to the persistent history and writes the full log to disk.
func (h *HistoryManager) Save(role string, content string, mindState string) error {
	h.messages = append(h.messages, Message{Author: role, Content: content, MindState: mindState, Stored: false})

	data, err := json.MarshalIndent(h.messages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize history: %w", err)
	}

	if err := os.WriteFile(h.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write history to disk: %w", err)
	}

	return nil
}

// GetMessages returns a copy of all messages currently stored in the history manager.
func (h *HistoryManager) GetMessages() []Message {
	return h.messages
}

// MarkStored flags messages in the range [start, end) as stored and writes the updated array back to disk.
func (h *HistoryManager) MarkStored(start, end int) error {
	if start < 0 || end > len(h.messages) || start > end {
		return fmt.Errorf("invalid range: [%d, %d)", start, end)
	}
	for i := start; i < end; i++ {
		h.messages[i].Stored = true
	}

	data, err := json.MarshalIndent(h.messages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize history: %w", err)
	}

	if err := os.WriteFile(h.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write history to disk: %w", err)
	}

	return nil
}
