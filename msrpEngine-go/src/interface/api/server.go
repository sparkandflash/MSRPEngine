package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"msrpengine/src/contextManager"
	"msrpengine/src/escalator"
	"msrpengine/src/utils"
)

type ChatInput struct {
	Message      string
	ResponseChan chan string
}

type Server struct {
	InputChan     chan<- ChatInput
	HistoryMgr    *contextManager.EventLogContext
	Sched         *escalator.Scheduler
	MindStateFunc func() string
	SessionMgr    *SessionManager
}

type ctxKey string

const sessionKey ctxKey = "sessionID"

func StartServer(inputChan chan<- ChatInput, historyMgr *contextManager.EventLogContext, sched *escalator.Scheduler, msFunc func() string) {
	if utils.Config.JWTSecret == "" {
		fmt.Println("System Error: API server JWT_SECRET is missing. Server will not start.")
		os.Exit(1)
	}

	sm := NewSessionManager(filepath.Join(utils.ResolvePath("Context"), "sessions.json"))

	s := &Server{
		InputChan:     inputChan,
		HistoryMgr:    historyMgr,
		Sched:         sched,
		MindStateFunc: msFunc,
		SessionMgr:    sm,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus) // Read-only polling for availability
	mux.HandleFunc("/init", s.handleInit) // Generates a new session JWT
	mux.HandleFunc("/ping", s.authMiddleware(s.handlePing)) // Heartbeat and connection grab
	mux.HandleFunc("/disconnect", s.authMiddleware(s.handleDisconnect)) // Explicit disconnect
	mux.HandleFunc("/getMessages", s.authMiddleware(s.handleGetMessages))
	mux.HandleFunc("/getMessageHistory", s.authMiddleware(s.handleGetMessageHistory))
	mux.HandleFunc("/sendMessage", s.authMiddleware(s.handleSendMessage))

	port := utils.Config.Port
	if os.Getenv("DEBUG") == "1" || os.Getenv("DEBUG") == "true" {
		fmt.Printf("\033[36m[System: Web API started on port %s]\033[0m\n", port)
	}
	if err := http.ListenAndServe(":"+port, s.corsMiddleware(mux)); err != nil {
		fmt.Printf("Web API Error: %v\n", err)
	}
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := os.Getenv("CORS_ORIGIN")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "online",
		"engine": "lyra",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// handleInit generates a new Session ID and returns a JWT to the client.
func (s *Server) handleInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if they passed a specific session_id to resume
	var payload struct {
		SessionID string `json:"session_id"`
	}
	// It's optional, so we ignore decode errors
	_ = json.NewDecoder(r.Body).Decode(&payload)

	sessionID := payload.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}
	
	// Create user session data entry
	s.SessionMgr.mu.Lock()
	s.SessionMgr.GetUser(sessionID)
	s.SessionMgr.mu.Unlock()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"session_id": sessionID,
		"exp":        time.Now().Add(time.Hour * 24 * 365).Unix(), // 1 year expiration
	})

	jwtSecret := []byte(utils.Config.JWTSecret)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": tokenString, "session_id": sessionID})
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid Authorization header", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]
		jwtSecret := []byte(utils.Config.JWTSecret)
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || claims["session_id"] == nil {
			http.Error(w, "Invalid token claims", http.StatusUnauthorized)
			return
		}

		sessionID := claims["session_id"].(string)
		ctx := context.WithValue(r.Context(), sessionKey, sessionID)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID := r.Context().Value(sessionKey).(string)

	// Try to grab lock if we don't have it
	err := s.SessionMgr.TryConnect(sessionID)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Update heartbeat
	err = s.SessionMgr.Heartbeat(sessionID)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "active"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Try extracting sessionID from optional Auth header if it exists
	sessionID := ""
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			tokenString := parts[1]
			jwtSecret := []byte(utils.Config.JWTSecret)
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				return jwtSecret, nil
			})
			if err == nil && token.Valid {
				if claims, ok := token.Claims.(jwt.MapClaims); ok && claims["session_id"] != nil {
					sessionID = claims["session_id"].(string)
				}
			}
		}
	}

	err := s.SessionMgr.CheckStatus(sessionID)
	
	responsePayload := map[string]interface{}{
		"limits": map[string]int{
			"global_daily_limit_mins": utils.Config.GlobalDailyLimitMinutes,
			"global_cooldown_mins":    utils.Config.GlobalCooldownMinutes,
			"user_daily_limit_mins":   utils.Config.UserDailyLimitMinutes,
			"max_chars_per_message":   utils.Config.MaxCharsPerMessage,
			"max_total_chars_per_user": utils.Config.MaxTotalCharsPerUser,
		},
	}

	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		responsePayload["error"] = err.Error()
		responsePayload["status"] = "busy_or_cooldown"
		json.NewEncoder(w).Encode(responsePayload)
		return
	}

	w.WriteHeader(http.StatusOK)
	responsePayload["status"] = "available"
	json.NewEncoder(w).Encode(responsePayload)
}

func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID := r.Context().Value(sessionKey).(string)
	s.SessionMgr.Disconnect(sessionID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "disconnected"})
}

func (s *Server) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	lastID := r.URL.Query().Get("last_id")
	allMsgs := s.HistoryMgr.GetMessages()
	
	var newMsgs []contextManager.InterfaceEvent
	for _, msg := range allMsgs {
		if lastID == "" || msg.ID > lastID {
			newMsgs = append(newMsgs, msg)
		}
	}

	hr := s.Sched.Engine.GetHeartrate()
	energy := s.Sched.Engine.GetMentalEnergy()
	mindStateStr := s.MindStateFunc()

	response := map[string]interface{}{
		"messages":      newMsgs,
		"heartrate":     hr,
		"mental_energy": energy,
		"mind_state":    mindStateStr,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleGetMessageHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	allMsgs := s.HistoryMgr.GetMessages()
	
	offsetStr := r.URL.Query().Get("offset")
	lengthStr := r.URL.Query().Get("length")
	
	offset := 0
	length := len(allMsgs)
	
	if offsetStr != "" {
		if val, err := strconv.Atoi(offsetStr); err == nil && val >= 0 {
			offset = val
		}
	}
	if lengthStr != "" {
		if val, err := strconv.Atoi(lengthStr); err == nil && val > 0 {
			length = val
		}
	}
	
	if offset > len(allMsgs) {
		offset = len(allMsgs)
	}
	
	end := offset + length
	if end > len(allMsgs) {
		end = len(allMsgs)
	}
	
	var paginatedMsgs []contextManager.InterfaceEvent
	if offset < end {
		paginatedMsgs = allMsgs[offset:end]
	} else {
		paginatedMsgs = []contextManager.InterfaceEvent{}
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"messages": paginatedMsgs,
		"total":    len(allMsgs),
	})
}

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID := r.Context().Value(sessionKey).(string)

	var payload struct {
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	msg := strings.TrimSpace(payload.Message)
	if msg == "" {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"reply": ""})
		return
	}

	// 1. Enforce Per-Message Character Limit
	if len(msg) > utils.Config.MaxCharsPerMessage {
		http.Error(w, fmt.Sprintf("Message exceeds maximum length of %d characters.", utils.Config.MaxCharsPerMessage), http.StatusBadRequest)
		return
	}

	// 2. Check Moderation (OpenAI)
	flagged, err := CheckModeration(msg)
	if err != nil {
		// Log error but optionally allow to pass, or block. We will block to be safe if strictly enforced.
		fmt.Printf("Moderation error: %v\n", err)
	}
	if flagged {
		http.Error(w, "Message flagged by moderation filters.", http.StatusForbidden)
		return
	}

	// 3. Record Message in Session Manager (Checks total char limits and active lock)
	err = s.SessionMgr.RecordMessage(sessionID, len(msg))
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	respChan := make(chan string, 1)
	s.InputChan <- ChatInput{
		Message:      msg,
		ResponseChan: respChan,
	}

	select {
	case reply := <-respChan:
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"reply": reply})
	case <-time.After(90 * time.Second):
		http.Error(w, "Request timed out", http.StatusGatewayTimeout)
	}
}
