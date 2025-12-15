package commands

import (
	"flag"
	"fmt"
	"os"

	"github.com/sjatkinson/threadkeeper/internal/config"
	"github.com/sjatkinson/threadkeeper/internal/store"
	"github.com/sjatkinson/threadkeeper/internal/task"
)

func RunRemove(args []string, ctx CommandContext) int {
	fs := flag.NewFlagSet(ctx.AppName+" remove", flag.ContinueOnError)
	fs.SetOutput(ctx.Err)
	fs.Usage = func() {
		fmt.Fprintln(ctx.Err, removeUsage(ctx.AppName))
	}

	var path string
	var force bool
	fs.StringVar(&path, "path", "", "custom workspace path")
	fs.BoolVar(&force, "force", false, "actually delete (required)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(ctx.Err)
		fmt.Fprintln(ctx.Err, removeUsage(ctx.AppName))
		return 2
	}

	ids := fs.Args()
	if len(ids) == 0 {
		fmt.Fprintf(ctx.Err, "Error: missing argument: task ID required\n")
		return 2
	}

	// Require --force flag
	if !force {
		fmt.Fprintf(ctx.Err, "Error: remove is a hard delete and requires --force\n")
		return 1
	}

	// Get paths and verify threads directory exists
	paths, err := config.GetPaths(path)
	if err != nil {
		fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	if _, err := os.Stat(paths.ThreadsDir); err != nil {
		fmt.Fprintf(ctx.Err, "Error: threads directory does not exist at %s. Run '%s init' first.\n", paths.ThreadsDir, ctx.AppName)
		return 1
	}

	// Load and resolve tasks
	st := store.NewFileStore(paths.ThreadsDir)
	var tasks []*task.Task
	for _, idStr := range ids {
		t, err := st.ResolveID(idStr)
		if err != nil {
			fmt.Fprintf(ctx.Err, "Error: %v\n", err)
			return 1
		}
		tasks = append(tasks, t)
	}

	// Delete each thread directory
	for _, t := range tasks {
		threadDir := store.ThreadPath(paths.ThreadsDir, t.ID)
		if _, err := os.Stat(threadDir); err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(ctx.Err, "Error: thread directory for task %s not found; skipping\n", t.ID)
				continue
			}
			fmt.Fprintf(ctx.Err, "Error: failed to check thread directory %s: %v\n", t.ID, err)
			continue
		}

		if err := os.RemoveAll(threadDir); err != nil {
			fmt.Fprintf(ctx.Err, "Error: failed to remove thread %s: %v\n", t.ID, err)
			continue
		}

		sidStr := "?"
		if t.ShortID != nil {
			sidStr = fmt.Sprintf("%d", *t.ShortID)
		}
		fmt.Fprintf(ctx.Out, "Removed task %s (%s)\n", sidStr, t.ID)
	}

	return 0
}

func removeUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s remove [--path <dir>] --force <id> [<id> ...]

Flags:
  --path <dir>   custom workspace path
  --force        actually delete (required)

`, app)
}
