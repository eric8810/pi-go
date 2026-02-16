package coding

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pi-go/ai"
)

func TestNewSessionManager_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")

	sm, err := NewSessionManager(dir)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}
	defer sm.Close()

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("expected directory to exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected a directory, got a file")
	}
}

func TestNewSession_CreatesFile(t *testing.T) {
	dir := t.TempDir()

	sm, err := NewSessionManager(dir)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}
	defer sm.Close()

	if err := sm.NewSession(); err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	sessionID := sm.SessionID()
	if sessionID == "" {
		t.Fatal("expected non-empty session ID")
	}
	if !strings.HasPrefix(sessionID, "session-") {
		t.Errorf("expected session ID to start with 'session-', got: %s", sessionID)
	}

	// Check file exists
	path := filepath.Join(dir, sessionID+".jsonl")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected session file to exist at %s: %v", path, err)
	}
}

func TestAppendMessage_WritesMessage(t *testing.T) {
	dir := t.TempDir()

	sm, err := NewSessionManager(dir)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}
	defer sm.Close()

	if err := sm.NewSession(); err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	msg := ai.NewUserMessage("hello")
	id, err := sm.AppendMessage(msg, "")
	if err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty message ID")
	}

	// Verify message is in the file
	path := filepath.Join(dir, sm.SessionID()+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read session file: %v", err)
	}
	if !strings.Contains(string(data), "hello") {
		t.Errorf("expected session file to contain 'hello', got: %s", string(data))
	}
}

func TestGetMessages_ReturnsInOrder(t *testing.T) {
	dir := t.TempDir()

	sm, err := NewSessionManager(dir)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}
	defer sm.Close()

	if err := sm.NewSession(); err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	msg1 := ai.NewUserMessage("first")
	id1, err := sm.AppendMessage(msg1, "")
	if err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	msg2 := ai.NewUserMessage("second")
	_, err = sm.AppendMessage(msg2, id1)
	if err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	msg3 := ai.NewUserMessage("third")
	_, err = sm.AppendMessage(msg3, "")
	if err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	messages, err := sm.GetMessages()
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}

	// Verify order
	texts := []string{"first", "second", "third"}
	for i, msg := range messages {
		um, ok := msg.(*ai.UserMessage)
		if !ok {
			t.Fatalf("message %d: expected *UserMessage, got %T", i, msg)
		}
		if len(um.Content) == 0 || um.Content[0].Text != texts[i] {
			t.Errorf("message %d: expected text %q, got %q", i, texts[i], um.Content[0].Text)
		}
	}
}

func TestListSessions_ReturnsSessionIDs(t *testing.T) {
	dir := t.TempDir()

	// Create two session files manually with distinct names to avoid timestamp collision
	id1 := "session-1000001"
	id2 := "session-1000002"
	for _, id := range []string{id1, id2} {
		f, err := os.Create(filepath.Join(dir, id+".jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
	}

	sm, err := NewSessionManager(dir)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}
	defer sm.Close()

	sessions, err := sm.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) < 2 {
		t.Fatalf("expected at least 2 sessions, got %d", len(sessions))
	}

	found := map[string]bool{}
	for _, s := range sessions {
		found[s] = true
	}
	if !found[id1] {
		t.Errorf("expected session %s in list", id1)
	}
	if !found[id2] {
		t.Errorf("expected session %s in list", id2)
	}
}

func TestSessionID_ReturnsCurrent(t *testing.T) {
	dir := t.TempDir()

	sm, err := NewSessionManager(dir)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}
	defer sm.Close()

	// Before creating a session, ID should be empty
	if sm.SessionID() != "" {
		t.Errorf("expected empty session ID before NewSession, got: %s", sm.SessionID())
	}

	if err := sm.NewSession(); err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	id := sm.SessionID()
	if id == "" {
		t.Fatal("expected non-empty session ID after NewSession")
	}
	if !strings.HasPrefix(id, "session-") {
		t.Errorf("expected session ID to start with 'session-', got: %s", id)
	}
}

func TestLoadSession_LoadsExisting(t *testing.T) {
	dir := t.TempDir()

	// Create a session and add a message
	sm, err := NewSessionManager(dir)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	if err := sm.NewSession(); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	sessionID := sm.SessionID()

	msg := ai.NewUserMessage("persisted message")
	_, err = sm.AppendMessage(msg, "")
	if err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	sm.Close()

	// Load the session in a new manager
	sm2, err := NewSessionManager(dir)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}
	defer sm2.Close()

	if err := sm2.LoadSession(sessionID); err != nil {
		t.Fatalf("LoadSession: %v", err)
	}

	if sm2.SessionID() != sessionID {
		t.Errorf("expected session ID %s, got %s", sessionID, sm2.SessionID())
	}

	messages, err := sm2.GetMessages()
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	um, ok := messages[0].(*ai.UserMessage)
	if !ok {
		t.Fatalf("expected *UserMessage, got %T", messages[0])
	}
	if len(um.Content) == 0 || um.Content[0].Text != "persisted message" {
		t.Errorf("expected 'persisted message', got: %+v", um.Content)
	}

	// Verify we can still append to the loaded session
	msg2 := ai.NewUserMessage("appended after load")
	_, err = sm2.AppendMessage(msg2, "")
	if err != nil {
		t.Fatalf("AppendMessage after load: %v", err)
	}

	messages, err = sm2.GetMessages()
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages after append, got %d", len(messages))
	}
}

func TestClose_NoError(t *testing.T) {
	dir := t.TempDir()

	sm, err := NewSessionManager(dir)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	if err := sm.NewSession(); err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	if err := sm.Close(); err != nil {
		t.Fatalf("Close: unexpected error: %v", err)
	}

	// Close again should be safe (file is nil now after first close sets it)
	// The second close calls Close on nil file, which returns nil.
}

func TestClose_WithoutSession(t *testing.T) {
	dir := t.TempDir()

	sm, err := NewSessionManager(dir)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	// Close without ever creating a session should not error
	if err := sm.Close(); err != nil {
		t.Fatalf("Close without session: unexpected error: %v", err)
	}
}
