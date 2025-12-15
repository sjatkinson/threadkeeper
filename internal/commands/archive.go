package commands

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sjatkinson/threadkeeper/internal/config"
	"github.com/sjatkinson/threadkeeper/internal/store"
	"github.com/sjatkinson/threadkeeper/internal/task"
)

func RunArchive(args []string, ctx CommandContext) int {
	fs := flag.NewFlagSet(ctx.AppName+" archive", flag.ContinueOnError)
	fs.SetOutput(ctx.Err)
	fs.Usage = func() {
		fmt.Fprintln(ctx.Err, archiveUsage(ctx.AppName))
	}

	var path string
	fs.StringVar(&path, "path", "", "custom workspace path")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(ctx.Err)
		fmt.Fprintln(ctx.Err, archiveUsage(ctx.AppName))
		return 2
	}

	ids := fs.Args()
	if len(ids) == 0 {
		fmt.Fprintf(ctx.Err, "Error: missing argument: task ID required\n")
		return 2
	}

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

	// Load and resolve tasks, continue on errors
	st := store.NewFileStore(paths.ThreadsDir)
	var tasks []*task.Task
	hasErrors := false

	for _, idStr := range ids {
		t, err := st.ResolveID(idStr)
		if err != nil {
			fmt.Fprintf(ctx.Err, "Error: failed to resolve ID %q: %v\n", idStr, err)
			hasErrors = true
			continue
		}
		tasks = append(tasks, t)
	}

	// Archive each task
	now := time.Now().UTC()
	for _, t := range tasks {
		// Capture short_id before removing it for output
		sidStr := "?"
		if t.ShortID != nil {
			sidStr = fmt.Sprintf("%d", *t.ShortID)
		}

		// Check if already archived
		if t.Status == task.StatusArchived {
			fmt.Fprintf(ctx.Err, "Warning: task %s (%s) is already archived\n", sidStr, t.ID)
			continue
		}

		// Archive the task
		t.Status = task.StatusArchived
		t.UpdatedAt = now
		// Remove short_id since it's only for open tasks
		t.ShortID = nil

		if err := st.Save(t); err != nil {
			fmt.Fprintf(ctx.Err, "Error: failed to save task %s (%s): %v\n", sidStr, t.ID, err)
			hasErrors = true
			continue
		}

		fmt.Fprintf(ctx.Out, "Archived task %s (%s)\n", sidStr, t.ID)
	}

	if hasErrors {
		return 1
	}

	return 0
}

func archiveUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s archive [--path <dir>] <id> [<id> ...]

Flags:
  --path <dir>   custom workspace path

`, app)
}
