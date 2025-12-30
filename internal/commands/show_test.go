package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestComputeCurrentAttachments(t *testing.T) {
	now := time.Now().UTC()
	time1 := now.Add(-2 * time.Hour).Format(time.RFC3339)
	time2 := now.Add(-1 * time.Hour).Format(time.RFC3339)
	time3 := now.Format(time.RFC3339)

	tests := []struct {
		name    string
		events  []AttachmentEvent
		want    []string // attachment IDs in expected order
		wantLen int
	}{
		{
			name:    "empty events",
			events:  []AttachmentEvent{},
			want:    []string{},
			wantLen: 0,
		},
		{
			name: "single add",
			events: []AttachmentEvent{
				{
					Op: "add",
					TS: time1,
					Att: Attachment{
						AttID: "att1",
						Kind:  "note",
						Name:  "note1",
					},
				},
			},
			want:    []string{"att1"},
			wantLen: 1,
		},
		{
			name: "add then remove",
			events: []AttachmentEvent{
				{
					Op: "add",
					TS: time1,
					Att: Attachment{
						AttID: "att1",
						Kind:  "note",
						Name:  "note1",
					},
				},
				{
					Op: "remove",
					TS: time2,
					Att: Attachment{
						AttID: "att1",
						Kind:  "note",
						Name:  "note1",
					},
				},
			},
			want:    []string{},
			wantLen: 0,
		},
		{
			name: "multiple adds",
			events: []AttachmentEvent{
				{
					Op: "add",
					TS: time1,
					Att: Attachment{
						AttID: "att1",
						Kind:  "note",
						Name:  "note1",
					},
				},
				{
					Op: "add",
					TS: time2,
					Att: Attachment{
						AttID: "att2",
						Kind:  "note",
						Name:  "note2",
					},
				},
				{
					Op: "add",
					TS: time3,
					Att: Attachment{
						AttID: "att3",
						Kind:  "note",
						Name:  "note3",
					},
				},
			},
			want:    []string{"att1", "att2", "att3"},
			wantLen: 3,
		},
		{
			name: "add, remove, add again",
			events: []AttachmentEvent{
				{
					Op: "add",
					TS: time1,
					Att: Attachment{
						AttID: "att1",
						Kind:  "note",
						Name:  "note1",
					},
				},
				{
					Op: "remove",
					TS: time2,
					Att: Attachment{
						AttID: "att1",
						Kind:  "note",
						Name:  "note1",
					},
				},
				{
					Op: "add",
					TS: time3,
					Att: Attachment{
						AttID: "att1",
						Kind:  "note",
						Name:  "note1-v2",
					},
				},
			},
			want:    []string{"att1"},
			wantLen: 1,
		},
		{
			name: "mixed add/remove with multiple attachments",
			events: []AttachmentEvent{
				{
					Op: "add",
					TS: time1,
					Att: Attachment{
						AttID: "att1",
						Kind:  "note",
						Name:  "note1",
					},
				},
				{
					Op: "add",
					TS: time2,
					Att: Attachment{
						AttID: "att2",
						Kind:  "note",
						Name:  "note2",
					},
				},
				{
					Op: "remove",
					TS: time3,
					Att: Attachment{
						AttID: "att1",
						Kind:  "note",
						Name:  "note1",
					},
				},
			},
			want:    []string{"att2"},
			wantLen: 1,
		},
		{
			name: "sorted by timestamp",
			events: []AttachmentEvent{
				{
					Op: "add",
					TS: time3, // Latest
					Att: Attachment{
						AttID: "att3",
						Kind:  "note",
						Name:  "note3",
					},
				},
				{
					Op: "add",
					TS: time1, // Earliest
					Att: Attachment{
						AttID: "att1",
						Kind:  "note",
						Name:  "note1",
					},
				},
				{
					Op: "add",
					TS: time2, // Middle
					Att: Attachment{
						AttID: "att2",
						Kind:  "note",
						Name:  "note2",
					},
				},
			},
			want:    []string{"att1", "att2", "att3"}, // Should be sorted by TS
			wantLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeCurrentAttachments(tt.events)

			if len(result) != tt.wantLen {
				t.Errorf("computeCurrentAttachments() returned %d attachments, want %d", len(result), tt.wantLen)
				return
			}

			if len(result) != len(tt.want) {
				t.Errorf("computeCurrentAttachments() returned %d attachments, want %d IDs", len(result), len(tt.want))
				return
			}

			for i, expectedID := range tt.want {
				if result[i].Att.AttID != expectedID {
					t.Errorf("computeCurrentAttachments()[%d].AttID = %q, want %q", i, result[i].Att.AttID, expectedID)
				}
			}

			// Verify sorting: timestamps should be in ascending order
			for i := 1; i < len(result); i++ {
				if result[i-1].TS > result[i].TS {
					t.Errorf("computeCurrentAttachments() not sorted: result[%d].TS (%s) > result[%d].TS (%s)",
						i-1, result[i-1].TS, i, result[i].TS)
				}
			}
		})
	}
}

func TestBlobPath(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		threadDir string
		blob      BlobRef
		want      string
		wantEmpty bool
	}{
		{
			name:      "valid sha256 hash",
			threadDir: tmpDir,
			blob: BlobRef{
				Algo: "sha256",
				Hash: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			},
			want:      filepath.Join(tmpDir, "blobs", "sha256", "ab", "cd", "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
			wantEmpty: false,
		},
		{
			name:      "short sha256 hash",
			threadDir: tmpDir,
			blob: BlobRef{
				Algo: "sha256",
				Hash: "abc123",
			},
			want:      filepath.Join(tmpDir, "blobs", "sha256", "ab", "c1", "abc123"),
			wantEmpty: false,
		},
		{
			name:      "unknown algorithm",
			threadDir: tmpDir,
			blob: BlobRef{
				Algo: "md5",
				Hash: "abc123",
			},
			want:      "",
			wantEmpty: true,
		},
		{
			name:      "empty algorithm",
			threadDir: tmpDir,
			blob: BlobRef{
				Algo: "",
				Hash: "abc123",
			},
			want:      "",
			wantEmpty: true,
		},
		{
			name:      "hash too short",
			threadDir: tmpDir,
			blob: BlobRef{
				Algo: "sha256",
				Hash: "ab", // Too short, need at least 4 chars
			},
			want:      "",
			wantEmpty: true,
		},
		{
			name:      "empty hash",
			threadDir: tmpDir,
			blob: BlobRef{
				Algo: "sha256",
				Hash: "",
			},
			want:      "",
			wantEmpty: true,
		},
		{
			name:      "exactly 4 character hash",
			threadDir: tmpDir,
			blob: BlobRef{
				Algo: "sha256",
				Hash: "abcd",
			},
			want:      filepath.Join(tmpDir, "blobs", "sha256", "ab", "cd", "abcd"),
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := blobPath(tt.threadDir, tt.blob)

			if tt.wantEmpty {
				if result != "" {
					t.Errorf("blobPath() = %q, want empty string", result)
				}
			} else {
				if result != tt.want {
					t.Errorf("blobPath() = %q, want %q", result, tt.want)
				}
				// Verify path structure
				if !strings.Contains(result, "blobs/sha256") {
					t.Errorf("blobPath() = %q, should contain 'blobs/sha256'", result)
				}
			}
		})
	}
}

func TestLoadAttachments_EventProcessing(t *testing.T) {
	tmpDir := t.TempDir()
	attachmentsPath := filepath.Join(tmpDir, "attachments.jsonl")

	// Create test JSONL file with add and remove events
	events := []string{
		`{"op":"add","ts":"2025-12-16T02:14:27Z","att":{"att_id":"att1","kind":"note","name":"note1","media_type":"text/markdown","blob":{"algo":"sha256","hash":"abc123"},"size":39}}`,
		`{"op":"add","ts":"2025-12-16T03:01:00Z","att":{"att_id":"att2","kind":"note","name":"note2","media_type":"text/markdown","blob":{"algo":"sha256","hash":"def456"},"size":42}}`,
		`{"op":"remove","ts":"2025-12-16T04:00:00Z","att":{"att_id":"att1","kind":"note","name":"note1","media_type":"text/markdown","blob":{"algo":"sha256","hash":"abc123"},"size":39}}`,
	}

	f, err := os.Create(attachmentsPath)
	if err != nil {
		t.Fatalf("Failed to create attachments.jsonl: %v", err)
	}
	for _, event := range events {
		if _, err := f.WriteString(event + "\n"); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}
	f.Close()

	// Load attachments
	loaded, err := loadAttachments(tmpDir)
	if err != nil {
		t.Fatalf("loadAttachments() error = %v", err)
	}

	if len(loaded) != 3 {
		t.Errorf("loadAttachments() returned %d events, want 3", len(loaded))
	}

	// Verify computeCurrentAttachments processes them correctly
	current := computeCurrentAttachments(loaded)
	if len(current) != 1 {
		t.Errorf("computeCurrentAttachments() returned %d attachments, want 1 (att1 removed)", len(current))
	}
	if len(current) > 0 && current[0].Att.AttID != "att2" {
		t.Errorf("computeCurrentAttachments()[0].AttID = %q, want 'att2'", current[0].Att.AttID)
	}
}

func TestLoadAttachments_MalformedLines(t *testing.T) {
	tmpDir := t.TempDir()
	attachmentsPath := filepath.Join(tmpDir, "attachments.jsonl")

	// Create test JSONL file with valid and malformed lines
	lines := []string{
		`{"op":"add","ts":"2025-12-16T02:14:27Z","att":{"att_id":"att1","kind":"note","name":"note1","blob":{"algo":"sha256","hash":"abc123"},"size":39}}`,
		`not valid json`,
		`{"op":"add","ts":"2025-12-16T03:01:00Z","att":{"att_id":"att2","kind":"note","name":"note2","blob":{"algo":"sha256","hash":"def456"},"size":42}}`,
		`{"incomplete":`,
		``, // Empty line
	}

	f, err := os.Create(attachmentsPath)
	if err != nil {
		t.Fatalf("Failed to create attachments.jsonl: %v", err)
	}
	for _, line := range lines {
		if _, err := f.WriteString(line + "\n"); err != nil {
			t.Fatalf("Failed to write line: %v", err)
		}
	}
	f.Close()

	// Load attachments - should skip malformed lines
	loaded, err := loadAttachments(tmpDir)
	if err != nil {
		t.Fatalf("loadAttachments() error = %v", err)
	}

	// Should have 2 valid events (att1 and att2)
	if len(loaded) != 2 {
		t.Errorf("loadAttachments() returned %d events, want 2 (malformed lines skipped)", len(loaded))
	}

	// Verify valid events are present
	ids := make(map[string]bool)
	for _, event := range loaded {
		ids[event.Att.AttID] = true
	}
	if !ids["att1"] {
		t.Error("loadAttachments() missing att1")
	}
	if !ids["att2"] {
		t.Error("loadAttachments() missing att2")
	}
}
