package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sjatkinson/threadkeeper/internal/config"
	"github.com/sjatkinson/threadkeeper/internal/store"
	"github.com/sjatkinson/threadkeeper/internal/task"
)

// AttachmentEvent represents an entry in attachments.jsonl
type AttachmentEvent struct {
	Op  string     `json:"op"`
	TS  string     `json:"ts"` // RFC3339 UTC timestamp
	Att Attachment `json:"att"`
}

// Attachment represents attachment metadata
type Attachment struct {
	AttID     string   `json:"att_id"`
	Kind      string   `json:"kind"` // "note" or "link"
	Name      string   `json:"name"`
	MediaType string   `json:"media_type,omitempty"` // Only for notes
	Blob      *BlobRef `json:"blob,omitempty"`       // Only for notes
	Size      int64    `json:"size,omitempty"`       // Only for notes
	URL       string   `json:"url,omitempty"`        // Only for links
	Label     string   `json:"label,omitempty"`      // Only for links (optional)
}

// BlobRef references a content-addressed blob
type BlobRef struct {
	Algo string `json:"algo"`
	Hash string `json:"hash"`
}

// captureEditorContent opens the user's editor and captures the content.
// Returns the content bytes, or an error.
func captureEditorContent() ([]byte, error) {
	// Determine editor: $TK_EDITOR > $EDITOR > vi
	editor := os.Getenv("TK_EDITOR")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "tk-attach-*.md")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up temp file

	// Write optional header comment
	header := "# Enter your note below. Lines starting with # are ignored.\n# Save and exit to attach, or delete all content to cancel.\n\n"
	if _, err := tmpFile.WriteString(header); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Open editor
	cmd := exec.Command(editor, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("editor exited with error: %w", err)
	}

	// Read the file content
	content, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read edited file: %w", err)
	}

	// Remove initial header comment lines (first few lines starting with #)
	// This removes the template header but preserves user content that starts with #
	lines := strings.Split(string(content), "\n")
	var bodyLines []string
	headerLinesRemoved := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Remove first 2-3 lines if they're comment lines (the header template)
		if i < 3 && strings.HasPrefix(trimmed, "#") {
			headerLinesRemoved++
			continue
		}
		bodyLines = append(bodyLines, line)
	}
	// If we removed header lines, also remove trailing empty line if present
	if headerLinesRemoved > 0 && len(bodyLines) > 0 && strings.TrimSpace(bodyLines[0]) == "" {
		bodyLines = bodyLines[1:]
	}
	body := strings.Join(bodyLines, "\n")

	// Check if content is empty or whitespace-only
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("note content is empty; attachment cancelled")
	}

	return []byte(body), nil
}

// storeBlob stores content as a content-addressed blob and returns the hash and size.
// Path: <thread-dir>/blobs/sha256/<first2>/<next2>/<hash>
func storeBlob(threadDir string, content []byte) (string, int64, error) {
	// Compute SHA-256 hash
	hash := sha256.Sum256(content)
	hashHex := hex.EncodeToString(hash[:])

	// Build nested path: blobs/sha256/<first2>/<next2>/<hash>
	first2 := hashHex[0:2]
	next2 := hashHex[2:4]
	blobPath := filepath.Join(threadDir, "blobs", "sha256", first2, next2, hashHex)

	// Check if blob already exists (idempotent)
	if _, err := os.Stat(blobPath); err == nil {
		// Blob exists, return hash and size
		info, err := os.Stat(blobPath)
		if err != nil {
			return "", 0, fmt.Errorf("failed to stat existing blob: %w", err)
		}
		return hashHex, info.Size(), nil
	}

	// Ensure parent directories exist
	if err := os.MkdirAll(filepath.Dir(blobPath), 0755); err != nil {
		return "", 0, fmt.Errorf("failed to create blob directory: %w", err)
	}

	// Write blob
	if err := os.WriteFile(blobPath, content, 0644); err != nil {
		return "", 0, fmt.Errorf("failed to write blob: %w", err)
	}

	return hashHex, int64(len(content)), nil
}

// appendAttachmentEvent appends an attachment event to attachments.jsonl.
// Returns error if write fails.
func appendAttachmentEvent(threadDir string, event AttachmentEvent) error {
	attachmentsPath := filepath.Join(threadDir, "attachments.jsonl")

	// Open file in append mode
	f, err := os.OpenFile(attachmentsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open attachments.jsonl: %w", err)
	}
	defer f.Close()

	// Encode event as JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal attachment event: %w", err)
	}

	// Write line + newline
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to write attachment event: %w", err)
	}
	if _, err := f.WriteString("\n"); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	return nil
}

// updateThreadAttachmentsLog updates thread.json to reference attachments.jsonl.
// Uses atomic write (temp file + rename). Loads existing task, updates it, and saves.
func updateThreadAttachmentsLog(threadsDir, threadID string) error {
	// Load existing task
	st := store.NewFileStore(threadsDir)
	t, err := st.GetByID(threadID)
	if err != nil {
		return fmt.Errorf("failed to load thread: %w", err)
	}

	// Update UpdatedAt timestamp
	t.UpdatedAt = time.Now().UTC()

	// Save task (this will write thread.json with all fields preserved)
	// Note: We need to add attachments_log field, but Task struct doesn't have it yet.
	// For now, we'll update it via JSON manipulation to preserve backward compatibility.
	threadPath := store.ThreadFilePath(threadsDir, threadID)

	// Read existing thread.json to preserve all fields
	data, err := os.ReadFile(threadPath)
	if err != nil {
		return fmt.Errorf("failed to read thread.json: %w", err)
	}

	// Parse JSON into map to preserve unknown fields
	var threadData map[string]interface{}
	if err := json.Unmarshal(data, &threadData); err != nil {
		return fmt.Errorf("failed to parse thread.json: %w", err)
	}

	// Add/update attachments_log field
	threadData["attachments_log"] = "attachments.jsonl"
	// Update updated_at to current time
	threadData["updated_at"] = t.UpdatedAt.Format(time.RFC3339)

	// Marshal back to JSON with indentation
	newData, err := json.MarshalIndent(threadData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal thread.json: %w", err)
	}

	// Atomic write: write to temp file, then rename
	tmpPath := threadPath + ".tmp"
	if err := os.WriteFile(tmpPath, newData, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, threadPath); err != nil {
		os.Remove(tmpPath) // Clean up on error
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

func RunAttach(args []string, ctx CommandContext) int {
	// Check for old positional syntax for better error messages
	// Old syntax: tk attach note <id> or tk attach link <id> <url>
	if len(args) >= 2 && (args[0] == "note" || args[0] == "link") {
		// Check if second arg looks like a positional (not a flag)
		if len(args) >= 2 && !strings.HasPrefix(args[1], "-") {
			// Old syntax detected
			if args[0] == "note" {
				_, _ = fmt.Fprintf(ctx.Err, "Error: attach now requires --id flag. Try: %s attach note --id %s\n", ctx.AppName, args[1])
				return 2
			} else {
				// link
				if len(args) >= 3 && !strings.HasPrefix(args[2], "-") {
					_, _ = fmt.Fprintf(ctx.Err, "Error: attach link now requires --id and --url flags. Try: %s attach link --id %s --url %s\n", ctx.AppName, args[1], args[2])
				} else {
					_, _ = fmt.Fprintf(ctx.Err, "Error: attach link now requires --id and --url flags. Try: %s attach link --id %s --url <url>\n", ctx.AppName, args[1])
				}
				return 2
			}
		}
	}

	// Parse flags for the subcommand (note or link)
	if len(args) == 0 {
		_, _ = fmt.Fprintln(ctx.Err, attachUsage(ctx.AppName))
		return 2
	}

	attachType := args[0]
	if attachType != "note" && attachType != "link" {
		_, _ = fmt.Fprintf(ctx.Err, "Error: invalid attachment type %q (must be 'note' or 'link')\n", attachType)
		_, _ = fmt.Fprintf(ctx.Err, "\n")
		_, _ = fmt.Fprintln(ctx.Err, attachUsage(ctx.AppName))
		return 2
	}

	// Create flag set for the subcommand
	subArgs := args[1:]
	fs := flag.NewFlagSet(ctx.AppName+" attach "+attachType, flag.ContinueOnError)
	fs.SetOutput(ctx.Err)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(ctx.Err, attachUsage(ctx.AppName))
	}

	var (
		id    string
		url   string
		label string
	)
	fs.StringVar(&id, "id", "", "thread handle or canonical id")
	if attachType == "link" {
		fs.StringVar(&url, "url", "", "URL to attach")
		fs.StringVar(&label, "label", "", "label for link")
	}

	if err := fs.Parse(subArgs); err != nil {
		_, _ = fmt.Fprintln(ctx.Err)
		_, _ = fmt.Fprintln(ctx.Err, attachUsage(ctx.AppName))
		return 2
	}

	// Check for positional arguments (old syntax)
	rest := fs.Args()
	if len(rest) > 0 {
		if attachType == "note" {
			_, _ = fmt.Fprintf(ctx.Err, "Error: attach now requires --id flag. Try: %s attach note --id %s\n", ctx.AppName, rest[0])
		} else {
			if len(rest) >= 2 {
				_, _ = fmt.Fprintf(ctx.Err, "Error: attach link now requires --id and --url flags. Try: %s attach link --id %s --url %s\n", ctx.AppName, rest[0], rest[1])
			} else if len(rest) == 1 {
				_, _ = fmt.Fprintf(ctx.Err, "Error: attach link now requires --id and --url flags. Try: %s attach link --id %s --url <url>\n", ctx.AppName, rest[0])
			}
		}
		return 2
	}

	// Validate required flags
	if id == "" {
		_, _ = fmt.Fprintf(ctx.Err, "Error: --id is required\n")
		_, _ = fmt.Fprintln(ctx.Err, attachUsage(ctx.AppName))
		return 2
	}

	if attachType == "note" {
		return runAttachNote(id, ctx.Path, ctx)
	}

	// Link attachment
	if url == "" {
		_, _ = fmt.Fprintf(ctx.Err, "Error: --url is required for link attachments\n")
		_, _ = fmt.Fprintln(ctx.Err, attachUsage(ctx.AppName))
		return 2
	}

	return runAttachLink(id, url, label, ctx.Path, ctx)
}

func runAttachNote(threadIDStr, path string, ctx CommandContext) int {

	// Get paths and verify threads directory exists
	paths, err := config.GetPaths(path)
	if err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	if _, err := os.Stat(paths.ThreadsDir); err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: threads directory does not exist at %s. Run '%s init' first.\n", paths.ThreadsDir, ctx.AppName)
		return 1
	}

	// Resolve thread ID
	st := store.NewFileStore(paths.ThreadsDir)
	t, err := st.ResolveID(threadIDStr)
	if err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	// Get thread directory path
	threadDir := store.ThreadPath(paths.ThreadsDir, t.ID)

	// Verify thread directory and thread.json exist
	threadJSONPath := store.ThreadFilePath(paths.ThreadsDir, t.ID)
	if _, err := os.Stat(threadJSONPath); err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: thread %s not found\n", t.ID)
		return 1
	}

	// Capture content from editor
	content, err := captureEditorContent()
	if err != nil {
		if err.Error() == "note content is empty; attachment cancelled" {
			_, _ = fmt.Fprintf(ctx.Err, "Note content is empty; attachment cancelled\n")
			return 0 // Not an error, user cancelled
		}
		_, _ = fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	// Store blob
	hashHex, size, err := storeBlob(threadDir, content)
	if err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: failed to store blob: %v\n", err)
		return 1
	}

	// Generate attachment ID
	attID, err := task.GenerateID()
	if err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: failed to generate attachment ID: %v\n", err)
		return 1
	}

	// Generate default name: note-YYYYMMDD-HHMMSS
	now := time.Now().UTC()
	name := fmt.Sprintf("note-%s", now.Format("20060102-150405"))

	// Create attachment event
	event := AttachmentEvent{
		Op: "add",
		TS: now.Format(time.RFC3339),
		Att: Attachment{
			AttID:     attID,
			Kind:      "note",
			Name:      name,
			MediaType: "text/markdown",
			Blob: &BlobRef{
				Algo: "sha256",
				Hash: hashHex,
			},
			Size: size,
		},
	}

	// Append to attachments.jsonl
	if err := appendAttachmentEvent(threadDir, event); err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: failed to append attachment event: %v\n", err)
		return 1
	}

	// Update thread.json to reference attachments.jsonl
	if err := updateThreadAttachmentsLog(paths.ThreadsDir, t.ID); err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: failed to update thread.json: %v\n", err)
		return 1
	}

	// Print success message
	_, _ = fmt.Fprintf(ctx.Out, "Attached note %s to %s (sha256:%s)\n", attID, t.ID, hashHex)

	return 0
}

func runAttachLink(threadIDStr, url, label, path string, ctx CommandContext) int {
	// Get paths and verify threads directory exists
	paths, err := config.GetPaths(path)
	if err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	if _, err := os.Stat(paths.ThreadsDir); err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: threads directory does not exist at %s. Run '%s init' first.\n", paths.ThreadsDir, ctx.AppName)
		return 1
	}

	// Resolve thread ID
	st := store.NewFileStore(paths.ThreadsDir)
	t, err := st.ResolveID(threadIDStr)
	if err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	// Get thread directory path
	threadDir := store.ThreadPath(paths.ThreadsDir, t.ID)

	// Verify thread directory and thread.json exist
	threadJSONPath := store.ThreadFilePath(paths.ThreadsDir, t.ID)
	if _, err := os.Stat(threadJSONPath); err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: thread %s not found\n", t.ID)
		return 1
	}

	// Generate attachment ID
	attID, err := task.GenerateID()
	if err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: failed to generate attachment ID: %v\n", err)
		return 1
	}

	// Generate default name from URL or label
	now := time.Now().UTC()
	var name string
	if label != "" {
		name = label
	} else {
		// Use URL hostname as name, or fallback to link-YYYYMMDD-HHMMSS
		name = fmt.Sprintf("link-%s", now.Format("20060102-150405"))
	}

	// Create attachment event
	event := AttachmentEvent{
		Op: "add",
		TS: now.Format(time.RFC3339),
		Att: Attachment{
			AttID: attID,
			Kind:  "link",
			Name:  name,
			URL:   url,
			Label: label,
		},
	}

	// Append to attachments.jsonl
	if err := appendAttachmentEvent(threadDir, event); err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: failed to append attachment event: %v\n", err)
		return 1
	}

	// Update thread.json to reference attachments.jsonl
	if err := updateThreadAttachmentsLog(paths.ThreadsDir, t.ID); err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: failed to update thread.json: %v\n", err)
		return 1
	}

	// Print success message
	if label != "" {
		_, _ = fmt.Fprintf(ctx.Out, "Attached link %s to %s: [%s] %s\n", attID, t.ID, label, url)
	} else {
		_, _ = fmt.Fprintf(ctx.Out, "Attached link %s to %s: %s\n", attID, t.ID, url)
	}

	return 0
}

func attachUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s attach note --id <thread-id>
  %s attach link --id <thread-id> --url <url> [--label <label>]

Attach context to a thread.

Types:
  note   Open editor, store content-addressed blob, record in attachments.jsonl.
  link   Record URL (and optional label) in attachments.jsonl.

Flags:
  --id <id>       thread handle or canonical id
  --url <url>     URL to attach [link only]
  --label <text>  label for link (pr, slack, jira, doc, etc.) [link only]

Environment variables:
  TK_EDITOR       editor to use (defaults to $EDITOR, then vi) [note only]
  EDITOR          editor to use (if TK_EDITOR not set) [note only]

Examples:
  %s attach note --id 1
  %s attach link --id 1 --url https://example.com/pr/123 --label pr
  %s attach link --id 1 --url https://slack.com/archives/C123

`, app, app, app, app, app)
}
