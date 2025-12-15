package commands

import (
	"flag"
	"fmt"
	"io"
	"os"
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

	// Display based on mode
	if all {
		displayFull(ctx.Out, t)
	} else {
		displayMinimal(ctx.Out, t)
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

// displayMinimal shows a minimal view: short_id + title (if open) or just title, then description.
func displayMinimal(out io.Writer, t *task.Task) {
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
}

// displayFull shows full metadata and details.
func displayFull(out io.Writer, t *task.Task) {
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
}
