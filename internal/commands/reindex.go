package commands

import (
	"flag"
	"fmt"
	"os"

	"github.com/sjatkinson/threadkeeper/internal/config"
	"github.com/sjatkinson/threadkeeper/internal/store"
	"github.com/sjatkinson/threadkeeper/internal/task"
)

func RunReindex(args []string, ctx CommandContext) int {
	fs := flag.NewFlagSet(ctx.AppName+" reindex", flag.ContinueOnError)
	fs.SetOutput(ctx.Err)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(ctx.Err, reindexUsage(ctx.AppName))
	}

	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintln(ctx.Err)
		_, _ = fmt.Fprintln(ctx.Err, reindexUsage(ctx.AppName))
		return 2
	}

	if len(fs.Args()) != 0 {
		_, _ = fmt.Fprintln(ctx.Err, reindexUsage(ctx.AppName))
		return 2
	}

	// Get paths and verify tasks directory exists
	paths, err := config.GetPaths(ctx.Path)
	if err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	if _, err := os.Stat(paths.ThreadsDir); err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: threads directory does not exist at %s. Run '%s init' first.\n", paths.ThreadsDir, ctx.AppName)
		return 1
	}

	// Load all tasks
	st := store.NewFileStore(paths.ThreadsDir)
	tasks, err := st.LoadAll()
	if err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: failed to load tasks: %v\n", err)
		return 1
	}

	if len(tasks) == 0 {
		_, _ = fmt.Fprintf(ctx.Out, "No tasks to reindex.\n")
		return 0
	}

	// Filter active tasks (already sorted by created_at then id from LoadAll)
	var activeTasks []*task.Task
	for _, t := range tasks {
		if t.Status == task.StatusOpen {
			activeTasks = append(activeTasks, t)
		}
	}

	// Assign 1..N short_ids to active tasks
	sid := 1
	for _, t := range activeTasks {
		sidVal := sid
		t.ShortID = &sidVal
		sid++
	}

	// Remove short_id from non-active tasks
	for _, t := range tasks {
		if t.Status != task.StatusOpen {
			t.ShortID = nil
		}
	}

	// Save all tasks back
	for _, t := range tasks {
		if err := st.Save(t); err != nil {
			_, _ = fmt.Fprintf(ctx.Err, "Error: failed to save task %s: %v\n", t.ID, err)
			return 1
		}
	}

	count := len(activeTasks)
	if count > 0 {
		_, _ = fmt.Fprintf(ctx.Out, "Reindexed %d active tasks with short IDs 1..%d\n", count, count)
	} else {
		_, _ = fmt.Fprintf(ctx.Out, "No active tasks to reindex.\n")
	}

	return 0
}

func reindexUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s reindex

`, app)
}
