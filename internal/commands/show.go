package commands

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
		fmt.Fprintln(ctx.Err, showUsage(ctx.AppName))
	}

	var path string
	var all bool
	fs.StringVar(&path, "path", "", "custom workspace path")
	fs.BoolVar(&all, "all", false, "show full metadata")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(ctx.Err)
		fmt.Fprintln(ctx.Err, showUsage(ctx.AppName))
		return 2
	}

	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintf(ctx.Err, "Error: missing argument: task ID required\n")
		return 2
	}

	idStr := rest[0]

	// Get paths and verify tasks directory exists
	paths, err := config.GetPaths(path)
	if err != nil {
		fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	if _, err := os.Stat(paths.ThreadsDir); err != nil {
		fmt.Fprintf(ctx.Err, "Error: threads directory does not exist at %s. Run '%s init' first.\n", paths.ThreadsDir, ctx.AppName)
		return 1
	}

	// Load and resolve task
	st := store.NewFileStore(paths.ThreadsDir)
	t, err := st.ResolveID(idStr)
	if err != nil {
		fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	// Get thread directory path
	threadDir := store.ThreadPath(paths.ThreadsDir, t.ID)

	// Load attachments
	attachments, err := loadAttachments(threadDir)
	if err != nil {
		if !os.IsNotExist(err) {
			// Only error if file exists but can't be read; missing file is OK
			fmt.Fprintf(ctx.Err, "Warning: failed to load attachments: %v\n", err)
		}
		// If file doesn't exist, attachments will be nil, set to empty slice
		attachments = []AttachmentEvent{}
	}

	// Display based on mode
	if all {
		displayFull(ctx.Out, t, attachments)
	} else {
		displayMinimal(ctx.Out, t, attachments)
	}

	return 0
}

func showUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s show [--path <dir>] [--all] <id>

Flags:
  --path <dir>   custom workspace path
  --all          show full metadata

`, app)
}

// loadAttachments reads and parses attachments.jsonl from a thread directory.
// Returns empty slice and nil error if file doesn't exist.
func loadAttachments(threadDir string) ([]AttachmentEvent, error) {
	attachmentsPath := filepath.Join(threadDir, "attachments.jsonl")
	f, err := os.Open(attachmentsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []AttachmentEvent{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var attachments []AttachmentEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event AttachmentEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Skip malformed lines but continue parsing
			continue
		}
		attachments = append(attachments, event)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return attachments, nil
}

// displayMinimal shows a minimal view: short_id + title (if open) or just title, then description, then attachments.
func displayMinimal(out io.Writer, t *task.Task, attachments []AttachmentEvent) {
	if t.Status == task.StatusOpen && t.ShortID != nil {
		fmt.Fprintf(out, "%d  %s\n", *t.ShortID, t.Title)
	} else {
		fmt.Fprintf(out, "%s\n", t.Title)
	}
	fmt.Fprintln(out)

	desc := strings.TrimSpace(t.Description)
	if desc == "" {
		fmt.Fprintln(out, "(no description)")
	} else {
		fmt.Fprintln(out, desc)
	}

	// Display attachments
	if len(attachments) > 0 {
		fmt.Fprintln(out)
		displayAttachmentsTable(out, attachments)
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
	// Filter to only "add" operations
	var addAttachments []AttachmentEvent
	for _, att := range attachments {
		if att.Op == "add" {
			addAttachments = append(addAttachments, att)
		}
	}

	if len(addAttachments) == 0 {
		fmt.Fprintln(out, "(no attachments)")
		return
	}

	// Print header
	fmt.Fprintf(out, "#  %-12s  %-6s  %-24s  %-6s  %s\n", "ID", "KIND", "NAME", "SIZE", "CREATED")

	// Print each attachment
	for i, att := range addAttachments {
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

		fmt.Fprintf(out, "%-2d %-12s  %-6s  %-24s  %-6s  %s\n",
			i+1, truncatedID, kind, name, sizeStr, created)
	}
}

// displayFull shows full metadata and details.
func displayFull(out io.Writer, t *task.Task, attachments []AttachmentEvent) {
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

	fmt.Fprintln(out, header)
	fmt.Fprintln(out, strings.Repeat("-", len(header)))

	// Status
	fmt.Fprintf(out, "Status : [%s] %s\n", flag, t.Status)

	// Project
	if t.Project != "" {
		fmt.Fprintf(out, "Project: %s\n", t.Project)
	}

	// Due date
	if t.DueAt != nil {
		fmt.Fprintf(out, "Due    : %s\n", t.DueAt.Format("2006-01-02"))
	}

	// Tags
	if len(t.Tags) > 0 {
		tagStrs := make([]string, len(t.Tags))
		for i, tag := range t.Tags {
			tagStrs[i] = "#" + tag
		}
		fmt.Fprintf(out, "Tags   : %s\n", strings.Join(tagStrs, " "))
	}

	// Created timestamp
	if !t.CreatedAt.IsZero() {
		fmt.Fprintf(out, "Created: %s\n", t.CreatedAt.Format(time.RFC3339))
	}

	// Updated timestamp
	if !t.UpdatedAt.IsZero() {
		fmt.Fprintf(out, "Updated: %s\n", t.UpdatedAt.Format(time.RFC3339))
	}

	// Title
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Title")
	fmt.Fprintln(out, "-----")
	fmt.Fprintln(out, t.Title)

	// Description
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Description")
	fmt.Fprintln(out, "-----------")
	desc := strings.TrimSpace(t.Description)
	if desc == "" {
		fmt.Fprintln(out, "(no description)")
	} else {
		fmt.Fprintln(out, desc)
	}

	// Attachments
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Attachments")
	fmt.Fprintln(out, "-----------")
	if len(attachments) == 0 {
		fmt.Fprintln(out, "(no attachments)")
	} else {
		displayAttachmentsTable(out, attachments)
	}
}
