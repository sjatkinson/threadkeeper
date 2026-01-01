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

func RunDone(args []string, ctx CommandContext) int {
	fs := flag.NewFlagSet(ctx.AppName+" done", flag.ContinueOnError)
	fs.SetOutput(ctx.Err)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(ctx.Err, doneUsage(ctx.AppName))
	}

	var path string
	fs.StringVar(&path, "path", "", "custom workspace path")

	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintln(ctx.Err)
		_, _ = fmt.Fprintln(ctx.Err, doneUsage(ctx.AppName))
		return 2
	}

	ids := fs.Args()
	if len(ids) == 0 {
		_, _ = fmt.Fprintf(ctx.Err, "Error: missing argument: task ID required\n")
		return 2
	}

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

	// Load and resolve tasks
	st := store.NewFileStore(paths.ThreadsDir)
	var tasks []*task.Task
	for _, idStr := range ids {
		t, err := st.ResolveID(idStr)
		if err != nil {
			_, _ = fmt.Fprintf(ctx.Err, "Error: %v\n", err)
			return 1
		}
		tasks = append(tasks, t)
	}

	// Mark each task as done
	now := time.Now().UTC()
	for _, t := range tasks {
		// Capture short_id before removing it for output
		sidStr := "?"
		if t.ShortID != nil {
			sidStr = fmt.Sprintf("%d", *t.ShortID)
		}

		t.Status = task.StatusDone
		t.UpdatedAt = now
		// Remove short_id since it's only for open tasks
		t.ShortID = nil

		if err := st.Save(t); err != nil {
			_, _ = fmt.Fprintf(ctx.Err, "Error: failed to save task %s: %v\n", t.ID, err)
			return 1
		}

		_, _ = fmt.Fprintf(ctx.Out, "Marked task %s (%s) as done\n", sidStr, t.ID)
	}

	return 0
}

func doneUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s done [--path <dir>] <id> [<id> ...]

Flags:
  --path <dir>   custom workspace path

`, app)
}
