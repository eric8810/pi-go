// Package coding provides the coding agent CLI application.
// It is the Go equivalent of @mariozechner/pi-coding-agent.
package coding

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"agentsdk/ai"
)

// SessionEntry is a single entry in the session JSONL file.
type SessionEntry struct {
	ID        string          `json:"id"`
	ParentID  string          `json:"parentId,omitempty"`
	Role      ai.Role         `json:"role"`
	Content   json.RawMessage `json:"content"`
	Timestamp time.Time       `json:"timestamp"`
	Metadata  map[string]any  `json:"metadata,omitempty"`
}

// SessionManager handles persistence of agent sessions as JSONL files.
type SessionManager struct {
	dir       string
	sessionID string
	file      *os.File
	entries   []SessionEntry
	nextID    int
}

// NewSessionManager creates a session manager that stores sessions in the given directory.
func NewSessionManager(dir string) (*SessionManager, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create session directory: %w", err)
	}

	return &SessionManager{
		dir:    dir,
		nextID: 1,
	}, nil
}

// DefaultSessionDir returns the default session directory (~/.pi/sessions).
func DefaultSessionDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".pi", "sessions")
	}
	return filepath.Join(home, ".pi", "sessions")
}

// NewSession creates a new session and opens its JSONL file.
func (sm *SessionManager) NewSession() error {
	sm.sessionID = fmt.Sprintf("session-%d", time.Now().UnixMilli())
	path := filepath.Join(sm.dir, sm.sessionID+".jsonl")

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create session file: %w", err)
	}

	sm.file = f
	sm.entries = nil
	sm.nextID = 1
	return nil
}

// LoadSession loads an existing session from a JSONL file.
func (sm *SessionManager) LoadSession(sessionID string) error {
	sm.sessionID = sessionID
	path := filepath.Join(sm.dir, sessionID+".jsonl")

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open session file: %w", err)
	}
	defer f.Close()

	sm.entries = nil
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max line

	for scanner.Scan() {
		var entry SessionEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue // skip malformed entries
		}
		sm.entries = append(sm.entries, entry)
		sm.nextID++
	}

	// Re-open for appending
	sm.file, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open session for appending: %w", err)
	}

	return nil
}

// AppendMessage appends a message to the current session.
func (sm *SessionManager) AppendMessage(msg ai.Message, parentID string) (string, error) {
	if sm.file == nil {
		return "", fmt.Errorf("no active session")
	}

	id := fmt.Sprintf("%d", sm.nextID)
	sm.nextID++

	msgBytes, err := ai.MarshalMessage(msg)
	if err != nil {
		return "", fmt.Errorf("marshal message: %w", err)
	}

	entry := SessionEntry{
		ID:        id,
		ParentID:  parentID,
		Role:      msg.GetRole(),
		Content:   msgBytes,
		Timestamp: msg.GetTimestamp(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("marshal entry: %w", err)
	}

	if _, err := fmt.Fprintf(sm.file, "%s\n", data); err != nil {
		return "", fmt.Errorf("write entry: %w", err)
	}

	sm.entries = append(sm.entries, entry)
	return id, nil
}

// GetMessages returns all messages from the current session in order.
func (sm *SessionManager) GetMessages() ([]ai.Message, error) {
	var messages []ai.Message
	for _, entry := range sm.entries {
		msg, err := ai.UnmarshalMessage(entry.Content)
		if err != nil {
			continue
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// ListSessions returns all available session IDs.
func (sm *SessionManager) ListSessions() ([]string, error) {
	entries, err := os.ReadDir(sm.dir)
	if err != nil {
		return nil, fmt.Errorf("read session directory: %w", err)
	}

	var sessions []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".jsonl" {
			name := entry.Name()
			sessions = append(sessions, name[:len(name)-6]) // strip .jsonl
		}
	}
	return sessions, nil
}

// SessionID returns the current session ID.
func (sm *SessionManager) SessionID() string {
	return sm.sessionID
}

// Close closes the session file.
func (sm *SessionManager) Close() error {
	if sm.file != nil {
		return sm.file.Close()
	}
	return nil
}
