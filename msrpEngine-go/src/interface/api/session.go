package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"msrpengine/src/utils"
)

// UserSessionData tracks metrics for a specific user.
type UserSessionData struct {
	SessionID        string `json:"session_id"`
	TotalTimeSeconds int    `json:"total_time_seconds"`
	TotalCharsSent   int    `json:"total_chars_sent"`
	MessageCount     int    `json:"message_count"`
	HistoryFile      string `json:"history_file"`
	LastActiveDate   string `json:"last_active_date"`
}

// SessionsFile is the structure saved to sessions.json.
type SessionsFile struct {
	GlobalDate             string                      `json:"global_date"`
	GlobalTotalTimeSeconds int                         `json:"global_total_time_seconds"`
	Users                  map[string]*UserSessionData `json:"users"`
}

// SessionManager manages global and per-user limits and concurrency.
type SessionManager struct {
	mu                  sync.Mutex
	filePath            string
	data                SessionsFile
	ActiveSession       string
	CooldownUntil       time.Time
	ActiveUserLastSeen  time.Time
	ActiveUserStartTime time.Time
}

// NewSessionManager creates a new session manager, loading existing data if available.
func NewSessionManager(path string) *SessionManager {
	sm := &SessionManager{
		filePath: path,
		data: SessionsFile{
			Users: make(map[string]*UserSessionData),
		},
	}
	sm.load()
	sm.checkDateReset()
	return sm
}

func (sm *SessionManager) load() {
	if _, err := os.Stat(sm.filePath); os.IsNotExist(err) {
		return
	}
	bytes, err := os.ReadFile(sm.filePath)
	if err == nil {
		json.Unmarshal(bytes, &sm.data)
		if sm.data.Users == nil {
			sm.data.Users = make(map[string]*UserSessionData)
		}
	}
}

func (sm *SessionManager) save() {
	bytes, err := json.MarshalIndent(sm.data, "", "  ")
	if err == nil {
		os.MkdirAll(filepath.Dir(sm.filePath), 0755)
		os.WriteFile(sm.filePath, bytes, 0644)
	}
}

// checkDateReset resets daily counters if the day has changed.
func (sm *SessionManager) checkDateReset() {
	today := time.Now().Format("2006-01-02")
	if sm.data.GlobalDate != today {
		sm.data.GlobalDate = today
		sm.data.GlobalTotalTimeSeconds = 0
		// We could optionally reset user times here, but they are checked per-user below.
	}
}

// GetUser ensures a user exists and resets their daily stats if needed.
func (sm *SessionManager) GetUser(sessionID string) *UserSessionData {
	today := time.Now().Format("2006-01-02")
	u, exists := sm.data.Users[sessionID]
	if !exists {
		u = &UserSessionData{
			SessionID:      sessionID,
			LastActiveDate: today,
		}
		sm.data.Users[sessionID] = u
	} else if u.LastActiveDate != today {
		u.TotalTimeSeconds = 0
		u.TotalCharsSent = 0
		u.LastActiveDate = today
	}
	return u
}

// TryConnect attempts to grab the active session lock.
func (sm *SessionManager) TryConnect(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.checkDateReset()

	// Global Limit
	if sm.data.GlobalTotalTimeSeconds >= utils.Config.GlobalDailyLimitMinutes*60 {
		return fmt.Errorf("Interaction limit reached for today.")
	}

	// Cooldown
	if time.Now().Before(sm.CooldownUntil) {
		return fmt.Errorf("Interface on cool down. Connect again after %d mins.", int(time.Until(sm.CooldownUntil).Minutes())+1)
	}

	// Single user concurrency
	if sm.ActiveSession != "" && sm.ActiveSession != sessionID {
		// Before rejecting, check if the current active session has timed out (heartbeat)
		if time.Since(sm.ActiveUserLastSeen) > time.Duration(utils.Config.HeartbeatTimeoutSeconds)*time.Second {
			// They timed out, kick them out and apply cooldown
			sm.disconnectUnlocked()
			return fmt.Errorf("Interface on cool down. Connect again after %d mins.", int(time.Until(sm.CooldownUntil).Minutes())+1)
		}
		return fmt.Errorf("Interface is busy.")
	}

	// Individual limit
	u := sm.GetUser(sessionID)
	if u.TotalTimeSeconds >= utils.Config.UserDailyLimitMinutes*60 {
		return fmt.Errorf("Session time limit reached, try again later.")
	}

	// Connect successfully
	if sm.ActiveSession == "" {
		sm.ActiveSession = sessionID
		sm.ActiveUserStartTime = time.Now()
		sm.ActiveUserLastSeen = time.Now()
	}
	return nil
}

// CheckStatus verifies if the interface is available without claiming the lock.
func (sm *SessionManager) CheckStatus(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.checkDateReset()

	// Global Limit
	if sm.data.GlobalTotalTimeSeconds >= utils.Config.GlobalDailyLimitMinutes*60 {
		return fmt.Errorf("Interaction limit reached for today.")
	}

	// Cooldown
	if time.Now().Before(sm.CooldownUntil) {
		return fmt.Errorf("Interface on cool down. Connect again after %d mins.", int(time.Until(sm.CooldownUntil).Minutes())+1)
	}

	// Single user concurrency
	if sm.ActiveSession != "" && sm.ActiveSession != sessionID {
		// Before rejecting, check if the current active session has timed out (heartbeat)
		if time.Since(sm.ActiveUserLastSeen) > time.Duration(utils.Config.HeartbeatTimeoutSeconds)*time.Second {
			// We can kick them out and apply cooldown even during a status check, to clean up dangling state
			sm.disconnectUnlocked()
			return fmt.Errorf("Interface on cool down. Connect again after %d mins.", int(time.Until(sm.CooldownUntil).Minutes())+1)
		}
		return fmt.Errorf("Interface is busy.")
	}

	// Individual limit
	if sessionID != "" {
		u := sm.GetUser(sessionID)
		if u.TotalTimeSeconds >= utils.Config.UserDailyLimitMinutes*60 {
			return fmt.Errorf("Session time limit reached, try again later.")
		}
	}

	return nil
}

// Heartbeat updates the last seen time for the active session.
func (sm *SessionManager) Heartbeat(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.ActiveSession != sessionID {
		return fmt.Errorf("Not active session")
	}

	sm.ActiveUserLastSeen = time.Now()
	
	// Update time spent
	elapsed := int(time.Since(sm.ActiveUserStartTime).Seconds())
	sm.ActiveUserStartTime = time.Now() // Reset start time for next tick
	
	sm.checkDateReset()
	sm.data.GlobalTotalTimeSeconds += elapsed
	u := sm.GetUser(sessionID)
	u.TotalTimeSeconds += elapsed
	sm.save()

	// Check if limits exceeded during heartbeat
	if sm.data.GlobalTotalTimeSeconds >= utils.Config.GlobalDailyLimitMinutes*60 {
		sm.disconnectUnlocked()
		return fmt.Errorf("Interaction limit reached for today.")
	}
	if u.TotalTimeSeconds >= utils.Config.UserDailyLimitMinutes*60 {
		sm.disconnectUnlocked()
		return fmt.Errorf("Session time limit reached, try again later.")
	}

	return nil
}

func (sm *SessionManager) RecordMessage(sessionID string, chars int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if sm.ActiveSession != sessionID {
		return fmt.Errorf("Not active session")
	}

	u := sm.GetUser(sessionID)
	if u.TotalCharsSent+chars > utils.Config.MaxTotalCharsPerUser {
		return fmt.Errorf("Character limit reached for today.")
	}

	u.TotalCharsSent += chars
	u.MessageCount++
	sm.save()
	return nil
}

// Disconnect voluntarily drops the session and triggers a cooldown.
func (sm *SessionManager) Disconnect(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.ActiveSession == sessionID {
		sm.disconnectUnlocked()
	}
}

func (sm *SessionManager) disconnectUnlocked() {
	if sm.ActiveSession != "" {
		// Apply global cooldown
		sm.CooldownUntil = time.Now().Add(time.Duration(utils.Config.GlobalCooldownMinutes) * time.Minute)
		sm.ActiveSession = ""
		sm.save()
	}
}
