package commands

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sjatkinson/threadkeeper/internal/config"
	"github.com/sjatkinson/threadkeeper/internal/store"
	"github.com/sjatkinson/threadkeeper/internal/task"
)

func RunReopen(args []string, ctx CommandContext) int {
	fs := flag.NewFlagSet(ctx.AppName+" reopen", flag.ContinueOnError)
	fs.SetOutput(ctx.Err)
	fs.Usage = func() {
		fmt.Fprintln(ctx.Err, reopenUsage(ctx.AppName))
	}

	// No flags - reopen doesn't accept any flags
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(ctx.Err)
		fmt.Fprintln(ctx.Err, reopenUsage(ctx.AppName))
		return 2
	}

	ids := fs.Args()
	if len(ids) == 0 {
		fmt.Fprintf(ctx.Err, "Error: missing argument: task ID required\n")
		return 2
	}

	// Get paths and verify tasks directory exists
	paths, err := config.GetPaths("")
	if err != nil {
		fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	if _, err := os.Stat(paths.ThreadsDir); err != nil {
		fmt.Fprintf(ctx.Err, "Error: threads directory does not exist at %s. Run '%s init' first.\n", paths.ThreadsDir, ctx.AppName)
		return 1
	}

	// Validate all IDs first - abort if any are missing
	st := store.NewFileStore(paths.ThreadsDir)
	var tasks []*task.Task
	var missingIDs []string

	for _, idStr := range ids {
		t, err := st.GetByID(idStr)
		if err != nil {
			missingIDs = append(missingIDs, idStr)
			continue
		}
		tasks = append(tasks, t)
	}

	// If any IDs are missing, abort without changing anything
	if len(missingIDs) > 0 {
		fmt.Fprintf(ctx.Err, "Error: unknown task IDs: %s\n", strings.Join(missingIDs, ", "))
		return 1
	}

	// Reopen each task
	now := time.Now().UTC()
	for _, t := range tasks {
		// If already active (open), treat as no-op
		if t.Status == task.StatusOpen {
			continue
		}

		// Change from inactive state to active
		t.Status = task.StatusOpen
		t.UpdatedAt = now

		// Ensure the task has a short_id (open tasks should have short_ids)
		if err := st.EnsureShortID(t); err != nil {
			fmt.Fprintf(ctx.Err, "Error: failed to assign short_id to task %s: %v\n", t.ID, err)
			return 1
		}

		if err := st.Save(t); err != nil {
			fmt.Fprintf(ctx.Err, "Error: failed to save task %s: %v\n", t.ID, err)
			return 1
		}

		// Print confirmation
		sidStr := "?"
		if t.ShortID != nil {
			sidStr = fmt.Sprintf("%d", *t.ShortID)
		}
		fmt.Fprintf(ctx.Out, "Reopened task %s (%s)\n", sidStr, t.ID)
	}

	return 0
}

func reopenUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s reopen <id> [<id> ...]

Reopen one or more tasks, changing their status from inactive (archived or done) to active.

`, app)
}
