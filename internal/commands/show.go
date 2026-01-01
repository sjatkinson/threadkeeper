package commands

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sjatkinson/threadkeeper/internal/config"
	"github.com/sjatkinson/threadkeeper/internal/store"
	"github.com/sjatkinson/threadkeeper/internal/task"
)

func RunShow(args []string, ctx CommandContext) int {
	fs := flag.NewFlagSet(ctx.AppName+" show", flag.ContinueOnError)
	fs.SetOutput(ctx.Err)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(ctx.Err, showUsage(ctx.AppName))
	}

	var path string
	var full bool
	var all bool // deprecated, use --full
	fs.StringVar(&path, "path", "", "custom workspace path")
	fs.BoolVar(&full, "full", false, "show full metadata and history")
	fs.BoolVar(&all, "all", false, "show full metadata (deprecated, use --full)")

	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintln(ctx.Err)
		_, _ = fmt.Fprintln(ctx.Err, showUsage(ctx.AppName))
		return 2
	}

	rest := fs.Args()
	if len(rest) != 1 {
		_, _ = fmt.Fprintf(ctx.Err, "Error: missing argument: task ID required\n")
		return 2
	}

	idStr := rest[0]

	// Get paths and verify tasks directory exists
	paths, err := config.GetPaths(path)
	if err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	if _, err := os.Stat(paths.ThreadsDir); err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: threads directory does not exist at %s. Run '%s init' first.\n", paths.ThreadsDir, ctx.AppName)
		return 1
	}

	// Load and resolve task
	st := store.NewFileStore(paths.ThreadsDir)
	t, err := st.ResolveID(idStr)
	if err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	// Get thread directory path
	threadDir := store.ThreadPath(paths.ThreadsDir, t.ID)

	// Load attachments
	attachments, err := loadAttachments(threadDir)
	if err != nil {
		if !os.IsNotExist(err) {
			// Only error if file exists but can't be read; missing file is OK
			_, _ = fmt.Fprintf(ctx.Err, "Warning: failed to load attachments: %v\n", err)
		}
		// If file doesn't exist, attachments will be nil, set to empty slice
		attachments = []AttachmentEvent{}
	}

	// Display based on mode
	if full || all {
		// In full mode, load with metadata to show malformed line warnings
		attResult, err := loadAttachmentsWithMetadata(threadDir)
		if err != nil && !os.IsNotExist(err) {
			_, _ = fmt.Fprintf(ctx.Err, "Warning: failed to load attachments: %v\n", err)
			attResult = &loadAttachmentsResult{Events: attachments, MalformedLine: 0}
		} else if err == nil {
			attachments = attResult.Events
		}
		displayFull(ctx.Out, t, attachments, attResult.MalformedLine)
	} else {
		displayContextual(ctx.Out, t, attachments, ctx.AppName)
	}

	return 0
}

func showUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s show [--path <dir>] [--full] <id>

Flags:
  --path <dir>   custom workspace path
  --full         show full metadata and history
  --all          show full metadata (deprecated, use --full)

`, app)
}

// loadAttachmentsResult holds both parsed events and metadata about parsing.
type loadAttachmentsResult struct {
	Events        []AttachmentEvent
	MalformedLine int // count of malformed lines encountered
}

// loadAttachments reads and parses attachments.jsonl from a thread directory.
// Returns empty slice and nil error if file doesn't exist.
func loadAttachments(threadDir string) ([]AttachmentEvent, error) {
	result, err := loadAttachmentsWithMetadata(threadDir)
	if err != nil {
		return nil, err
	}
	return result.Events, nil
}

// loadAttachmentsWithMetadata reads attachments.jsonl and returns events plus metadata.
// This is used when we need to track malformed lines for warnings.
func loadAttachmentsWithMetadata(threadDir string) (*loadAttachmentsResult, error) {
	attachmentsPath := filepath.Join(threadDir, "attachments.jsonl")
	f, err := os.Open(attachmentsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &loadAttachmentsResult{Events: []AttachmentEvent{}, MalformedLine: 0}, nil
		}
		return nil, err
	}
	defer f.Close()

	var attachments []AttachmentEvent
	malformedCount := 0
	scanner := bufio.NewScanner(f)
	// Increase buffer size for large JSONL events (default is 64KB, increase to 1MB)
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, 0, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event AttachmentEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Skip malformed lines but continue parsing
			malformedCount++
			continue
		}
		attachments = append(attachments, event)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &loadAttachmentsResult{Events: attachments, MalformedLine: malformedCount}, nil
}

// computeCurrentAttachments processes JSONL events and returns active attachments
// sorted by timestamp (stable ordering for indexing).
// Handles add/remove operations: only attachments that have been added and not removed are returned.
func computeCurrentAttachments(events []AttachmentEvent) []AttachmentEvent {
	active := make(map[string]AttachmentEvent) // keyed by att_id

	for _, event := range events {
		switch event.Op {
		case "add":
			active[event.Att.AttID] = event
		case "remove":
			delete(active, event.Att.AttID)
		}
	}

	// Convert to slice and sort by timestamp
	result := make([]AttachmentEvent, 0, len(active))
	for _, event := range active {
		result = append(result, event)
	}

	// Sort by TS (RFC3339 string comparison works for chronological order)
	sort.Slice(result, func(i, j int) bool {
		return result[i].TS < result[j].TS
	})

	return result
}

// displayAttachmentsHistory displays all attachment events in chronological order.
// This is used in full view to show complete history including removed attachments.
func displayAttachmentsHistory(out io.Writer, events []AttachmentEvent) {
	if len(events) == 0 {
		_, _ = fmt.Fprintln(out, "(no attachment events)")
		return
	}

	// Print header
	_, _ = fmt.Fprintf(out, "#  %-8s  %-12s  %-6s  %-24s  %-6s  %s\n", "OP", "ID", "KIND", "NAME", "SIZE", "CREATED")

	// Print each event in chronological order
	for i, event := range events {
		truncatedID := truncateID(event.Att.AttID)
		op := event.Op
		kind := event.Att.Kind
		name := event.Att.Name

		// Format size: show raw bytes for notes, "-" for others
		var sizeStr string
		if event.Att.Kind == "note" {
			sizeStr = fmt.Sprintf("%d", event.Att.Size)
		} else {
			sizeStr = "-"
		}

		created := formatAttachmentDate(event.TS)

		_, _ = fmt.Fprintf(out, "%-2d %-8s  %-12s  %-6s  %-24s  %-6s  %s\n",
			i+1, op, truncatedID, kind, name, sizeStr, created)
	}
}

// blobPath computes the filesystem path for a blob given thread directory and BlobRef.
// Returns empty string if algorithm is not supported.
// Path format: <thread-dir>/blobs/<algo>/<first2>/<next2>/<hash>
func blobPath(threadDir string, blob BlobRef) string {
	if blob.Algo != "sha256" {
		return "" // Unknown algorithm
	}
	if len(blob.Hash) < 4 {
		return "" // Hash too short
	}
	first2 := blob.Hash[0:2]
	next2 := blob.Hash[2:4]
	return filepath.Join(threadDir, "blobs", "sha256", first2, next2, blob.Hash)
}

// displayContextual shows a contextual glance: header with key fields, description if present, attachments if present.
func displayContextual(out io.Writer, t *task.Task, attachments []AttachmentEvent, appName string) {
	// Header: Task ID
	var headerParts []string
	if t.ShortID != nil {
		headerParts = append(headerParts, fmt.Sprintf("Task %d", *t.ShortID))
	}
	headerParts = append(headerParts, fmt.Sprintf("(%s)", t.ID))
	_, _ = fmt.Fprintf(out, "%s\n", strings.Join(headerParts, " "))

	// Metadata: Status, Project, Due
	var metaParts []string
	metaParts = append(metaParts, fmt.Sprintf("Status: %s", t.Status))
	if t.Project != "" {
		metaParts = append(metaParts, fmt.Sprintf("Project: %s", t.Project))
	}
	if t.DueAt != nil {
		metaParts = append(metaParts, fmt.Sprintf("Due: %s", t.DueAt.Format("2006-01-02")))
	}
	if len(metaParts) > 0 {
		_, _ = fmt.Fprintf(out, "%s\n", strings.Join(metaParts, " | "))
	}
	_, _ = fmt.Fprintln(out)

	// Description (only if present)
	desc := strings.TrimSpace(t.Description)
	if desc != "" {
		_, _ = fmt.Fprintln(out, "Description")
		_, _ = fmt.Fprintln(out, strings.Repeat("-", 11))
		_, _ = fmt.Fprintln(out, desc)
		_, _ = fmt.Fprintln(out)
	}

	// Attachments (only if present)
	currentAtts := computeCurrentAttachments(attachments)
	if len(currentAtts) > 0 {
		_, _ = fmt.Fprintln(out, "Attachments")
		_, _ = fmt.Fprintln(out, strings.Repeat("-", 11))
		for i, att := range currentAtts {
			kind := att.Att.Kind
			name := att.Att.Name

			// Format size: show raw bytes for notes, "-" for others
			var sizeStr string
			if att.Att.Kind == "note" {
				sizeStr = fmt.Sprintf("%d B", att.Att.Size)
			} else {
				sizeStr = "-"
			}

			created := formatAttachmentDate(att.TS)

			// Format: "N. name (kind, size, date)  open: tk open <id> --att N"
			_, _ = fmt.Fprintf(out, "%d. %s (%s, %s, %s)  open: %s open %s --att %d\n",
				i+1, name, kind, sizeStr, created, appName, t.ID, i+1)
		}
	}
}

// formatSize formats a byte size in human-readable format.
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// truncateID truncates an ID to show first 6 characters and last 4, with ellipsis.
func truncateID(id string) string {
	if len(id) <= 10 {
		return id
	}
	return id[:6] + "…" + id[len(id)-4:]
}

// formatAttachmentDate formats a timestamp for attachment display.
func formatAttachmentDate(tsStr string) string {
	if tsStr == "" {
		return "-"
	}
	ts, err := time.Parse(time.RFC3339, tsStr)
	if err != nil {
		return "-"
	}
	return ts.Format("2006-01-02 15:04Z")
}

// displayAttachmentsTable displays attachments in a compact table format.
func displayAttachmentsTable(out io.Writer, attachments []AttachmentEvent) {
	// Compute current attachments (handles add/remove operations)
	currentAtts := computeCurrentAttachments(attachments)

	if len(currentAtts) == 0 {
		_, _ = fmt.Fprintln(out, "(no attachments)")
		return
	}

	// Print header
	_, _ = fmt.Fprintf(out, "#  %-12s  %-6s  %-24s  %-6s  %s\n", "ID", "KIND", "NAME", "SIZE", "CREATED")

	// Print each attachment
	for i, att := range currentAtts {
		truncatedID := truncateID(att.Att.AttID)
		kind := att.Att.Kind
		name := att.Att.Name

		// Format size: show raw bytes for notes, "-" for others
		var sizeStr string
		if att.Att.Kind == "note" {
			sizeStr = fmt.Sprintf("%d", att.Att.Size)
		} else {
			sizeStr = "-"
		}

		created := formatAttachmentDate(att.TS)

		_, _ = fmt.Fprintf(out, "%-2d %-12s  %-6s  %-24s  %-6s  %s\n",
			i+1, truncatedID, kind, name, sizeStr, created)
	}
}

// displayFull shows full metadata and details.
func displayFull(out io.Writer, t *task.Task, attachments []AttachmentEvent, malformedLineCount int) {
	// Status flag mapping
	flagMap := map[task.Status]string{
		task.StatusOpen:     " ",
		task.StatusDone:     "x",
		task.StatusArchived: "-",
	}
	flag := flagMap[t.Status]
	if flag == "" {
		flag = "?"
	}

	// Build header
	var header string
	if t.Status == task.StatusOpen && t.ShortID != nil {
		header = fmt.Sprintf("Task %d (%s)", *t.ShortID, t.ID)
	} else {
		header = fmt.Sprintf("Task (%s)", t.ID)
	}

	_, _ = fmt.Fprintln(out, header)
	_, _ = fmt.Fprintln(out, strings.Repeat("-", len(header)))

	// Warn about malformed lines if any
	if malformedLineCount > 0 {
		_, _ = fmt.Fprintf(out, "Warning: %d malformed line(s) in attachments.jsonl were skipped\n", malformedLineCount)
		_, _ = fmt.Fprintln(out)
	}

	// Status
	_, _ = fmt.Fprintf(out, "Status : [%s] %s\n", flag, t.Status)

	// Project
	if t.Project != "" {
		_, _ = fmt.Fprintf(out, "Project: %s\n", t.Project)
	}

	// Due date
	if t.DueAt != nil {
		_, _ = fmt.Fprintf(out, "Due    : %s\n", t.DueAt.Format("2006-01-02"))
	}

	// Tags
	if len(t.Tags) > 0 {
		tagStrs := make([]string, len(t.Tags))
		for i, tag := range t.Tags {
			tagStrs[i] = "#" + tag
		}
		_, _ = fmt.Fprintf(out, "Tags   : %s\n", strings.Join(tagStrs, " "))
	}

	// Created timestamp
	if !t.CreatedAt.IsZero() {
		_, _ = fmt.Fprintf(out, "Created: %s\n", t.CreatedAt.Format(time.RFC3339))
	}

	// Updated timestamp
	if !t.UpdatedAt.IsZero() {
		_, _ = fmt.Fprintf(out, "Updated: %s\n", t.UpdatedAt.Format(time.RFC3339))
	}

	// Title
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "Title")
	_, _ = fmt.Fprintln(out, "-----")
	_, _ = fmt.Fprintln(out, t.Title)

	// Description
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "Description")
	_, _ = fmt.Fprintln(out, "-----------")
	desc := strings.TrimSpace(t.Description)
	if desc == "" {
		_, _ = fmt.Fprintln(out, "(no description)")
	} else {
		_, _ = fmt.Fprintln(out, desc)
	}

	// Attachments
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "Attachments")
	_, _ = fmt.Fprintln(out, "-----------")
	if len(attachments) == 0 {
		_, _ = fmt.Fprintln(out, "(no attachments)")
	} else {
		// Full view shows all events (history), not just current attachments
		displayAttachmentsHistory(out, attachments)
	}
}
