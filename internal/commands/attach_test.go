package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sjatkinson/threadkeeper/internal/store"
	"github.com/sjatkinson/threadkeeper/internal/task"
)

func TestStoreBlob(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "threadkeeper-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := []byte("Test note content\nLine 2")
	hashHex, size, err := storeBlob(tmpDir, content)
	if err != nil {
		t.Fatalf("storeBlob() error = %v", err)
	}

	// Verify hash is correct
	expectedHash := sha256.Sum256(content)
	expectedHashHex := hex.EncodeToString(expectedHash[:])
	if hashHex != expectedHashHex {
		t.Errorf("storeBlob() hash = %v, want %v", hashHex, expectedHashHex)
	}

	// Verify size
	if size != int64(len(content)) {
		t.Errorf("storeBlob() size = %v, want %v", size, len(content))
	}

	// Verify blob file exists at correct path
	first2 := hashHex[0:2]
	next2 := hashHex[2:4]
	expectedPath := filepath.Join(tmpDir, "blobs", "sha256", first2, next2, hashHex)
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("Blob file not found at expected path %s: %v", expectedPath, err)
	}

	// Verify idempotency - storing again should return same hash and size
	hashHex2, size2, err := storeBlob(tmpDir, content)
	if err != nil {
		t.Fatalf("storeBlob() second call error = %v", err)
	}
	if hashHex2 != hashHex {
		t.Errorf("storeBlob() second call hash = %v, want %v", hashHex2, hashHex)
	}
	if size2 != size {
		t.Errorf("storeBlob() second call size = %v, want %v", size2, size)
	}
}

func TestAppendAttachmentEvent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "threadkeeper-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	event := AttachmentEvent{
		Op: "add",
		TS: time.Now().UTC().Format(time.RFC3339),
		Att: Attachment{
			AttID:     "01TEST123456789",
			Kind:      "note",
			Name:      "test-note",
			MediaType: "text/markdown",
			Blob: &BlobRef{
				Algo: "sha256",
				Hash: "abc123",
			},
			Size: 42,
		},
	}

	// First append
	if err := appendAttachmentEvent(tmpDir, event); err != nil {
		t.Fatalf("appendAttachmentEvent() error = %v", err)
	}

	// Verify file exists
	attachmentsPath := filepath.Join(tmpDir, "attachments.jsonl")
	data, err := os.ReadFile(attachmentsPath)
	if err != nil {
		t.Fatalf("Failed to read attachments.jsonl: %v", err)
	}

	// Parse first line
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Errorf("Expected 1 line, got %d", len(lines))
	}

	var parsedEvent AttachmentEvent
	if err := json.Unmarshal([]byte(lines[0]), &parsedEvent); err != nil {
		t.Fatalf("Failed to parse event JSON: %v", err)
	}

	// Verify fields
	if parsedEvent.Op != "add" {
		t.Errorf("Event Op = %v, want 'add'", parsedEvent.Op)
	}
	if parsedEvent.Att.AttID != event.Att.AttID {
		t.Errorf("Event AttID = %v, want %v", parsedEvent.Att.AttID, event.Att.AttID)
	}
	if parsedEvent.Att.Kind != "note" {
		t.Errorf("Event Kind = %v, want 'note'", parsedEvent.Att.Kind)
	}
	if parsedEvent.Att.Blob == nil {
		t.Errorf("Event Blob is nil, want non-nil")
	} else {
		if parsedEvent.Att.Blob.Algo != "sha256" {
			t.Errorf("Event Blob.Algo = %v, want 'sha256'", parsedEvent.Att.Blob.Algo)
		}
		if parsedEvent.Att.Blob.Hash != "abc123" {
			t.Errorf("Event Blob.Hash = %v, want 'abc123'", parsedEvent.Att.Blob.Hash)
		}
	}
	if parsedEvent.Att.Size != 42 {
		t.Errorf("Event Size = %v, want 42", parsedEvent.Att.Size)
	}

	// Second append (verify append mode works)
	event2 := event
	event2.Att.AttID = "01TEST987654321"
	if err := appendAttachmentEvent(tmpDir, event2); err != nil {
		t.Fatalf("appendAttachmentEvent() second call error = %v", err)
	}

	// Verify both events are present
	data2, err := os.ReadFile(attachmentsPath)
	if err != nil {
		t.Fatalf("Failed to read attachments.jsonl after second append: %v", err)
	}

	lines2 := strings.Split(strings.TrimSpace(string(data2)), "\n")
	if len(lines2) != 2 {
		t.Errorf("Expected 2 lines after second append, got %d", len(lines2))
	}
}

func TestUpdateThreadAttachmentsLog(t *testing.T) {
	// Set up test environment
	originalEnv := os.Getenv("THREADKEEPER_WORKSPACE")
	defer os.Setenv("THREADKEEPER_WORKSPACE", originalEnv)

	tmpDir, err := os.MkdirTemp("", "threadkeeper-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.Setenv("THREADKEEPER_WORKSPACE", tmpDir)

	threadsDir := filepath.Join(tmpDir, "threads")
	if err := os.MkdirAll(threadsDir, 0755); err != nil {
		t.Fatalf("Failed to create threads dir: %v", err)
	}

	// Create a test thread
	threadID, err := task.GenerateID()
	if err != nil {
		t.Fatalf("Failed to generate thread ID: %v", err)
	}

	threadDir := store.ThreadPath(threadsDir, threadID)
	if err := os.MkdirAll(threadDir, 0755); err != nil {
		t.Fatalf("Failed to create thread dir: %v", err)
	}

	// Create initial thread.json
	now := time.Now().UTC()
	initialTask := &task.Task{
		ID:        threadID,
		Title:     "Test Thread",
		Status:    task.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
		Tags:      []string{"test"},
	}

	st := store.NewFileStore(threadsDir)
	if err := st.Save(initialTask); err != nil {
		t.Fatalf("Failed to save initial task: %v", err)
	}

	// Update attachments log
	if err := updateThreadAttachmentsLog(threadsDir, threadID); err != nil {
		t.Fatalf("updateThreadAttachmentsLog() error = %v", err)
	}

	// Verify thread.json was updated
	threadPath := store.ThreadFilePath(threadsDir, threadID)
	data, err := os.ReadFile(threadPath)
	if err != nil {
		t.Fatalf("Failed to read thread.json: %v", err)
	}

	var threadData map[string]interface{}
	if err := json.Unmarshal(data, &threadData); err != nil {
		t.Fatalf("Failed to parse thread.json: %v", err)
	}

	// Verify attachments_log field was added
	attachmentsLog, ok := threadData["attachments_log"]
	if !ok {
		t.Error("attachments_log field not found in thread.json")
	}
	if attachmentsLog != "attachments.jsonl" {
		t.Errorf("attachments_log = %v, want 'attachments.jsonl'", attachmentsLog)
	}

	// Verify other fields are preserved
	if threadData["id"] != threadID {
		t.Errorf("id field was modified: %v", threadData["id"])
	}
	if threadData["title"] != "Test Thread" {
		t.Errorf("title field was modified: %v", threadData["title"])
	}
	if tags, ok := threadData["tags"].([]interface{}); !ok || len(tags) != 1 {
		t.Errorf("tags field was modified: %v", threadData["tags"])
	}
}

func TestBlobPathComputation(t *testing.T) {
	// Test that blob path computation follows the nested structure
	hashHex := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	first2 := hashHex[0:2]
	next2 := hashHex[2:4]

	if first2 != "ab" {
		t.Errorf("first2 = %v, want 'ab'", first2)
	}
	if next2 != "cd" {
		t.Errorf("next2 = %v, want 'cd'", next2)
	}

	expectedPath := filepath.Join("blobs", "sha256", first2, next2, hashHex)
	if !strings.Contains(expectedPath, "blobs/sha256/ab/cd/") {
		t.Errorf("Blob path does not follow expected structure: %v", expectedPath)
	}
}
