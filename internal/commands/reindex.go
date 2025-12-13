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
		fmt.Fprintln(ctx.Err, reindexUsage(ctx.AppName))
	}

	var path string
	fs.StringVar(&path, "path", "", "custom workspace path")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(ctx.Err)
		fmt.Fprintln(ctx.Err, reindexUsage(ctx.AppName))
		return 2
	}

	if len(fs.Args()) != 0 {
		fmt.Fprintln(ctx.Err, reindexUsage(ctx.AppName))
		return 2
	}

	// Get paths and verify tasks directory exists
	paths, err := config.GetPaths(path)
	if err != nil {
		fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	if _, err := os.Stat(paths.TasksDir); err != nil {
		fmt.Fprintf(ctx.Err, "Error: tasks directory does not exist at %s. Run '%s init' first.\n", paths.TasksDir, ctx.AppName)
		return 1
	}

	// Load all tasks
	st := store.NewFileStore(paths.TasksDir)
	tasks, err := st.LoadAll()
	if err != nil {
		fmt.Fprintf(ctx.Err, "Error: failed to load tasks: %v\n", err)
		return 1
	}

	if len(tasks) == 0 {
		fmt.Fprintf(ctx.Out, "No tasks to reindex.\n")
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
			fmt.Fprintf(ctx.Err, "Error: failed to save task %s: %v\n", t.ID, err)
			return 1
		}
	}

	count := len(activeTasks)
	if count > 0 {
		fmt.Fprintf(ctx.Out, "Reindexed %d active tasks with short IDs 1..%d\n", count, count)
	} else {
		fmt.Fprintf(ctx.Out, "No active tasks to reindex.\n")
	}

	return 0
}

func reindexUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s reindex [--path <dir>]

Flags:
  --path <dir>   custom workspace path

`, app)
}
