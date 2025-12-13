package commands

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sjatkinson/threadkeeper/internal/config"
	"github.com/sjatkinson/threadkeeper/internal/store"
	"github.com/sjatkinson/threadkeeper/internal/task"
)

func RunList(args []string, ctx CommandContext) int {
	fs := flag.NewFlagSet(ctx.AppName+" list", flag.ContinueOnError)
	fs.SetOutput(ctx.Err)
	fs.Usage = func() {
		fmt.Fprintln(ctx.Err, listUsage(ctx.AppName))
	}

	var (
		path    string
		all     bool
		project string
		status  string
		limit   int
		tag     string
	)

	fs.StringVar(&path, "path", "", "custom workspace path")
	fs.BoolVar(&all, "all", false, "show all tasks")
	fs.BoolVar(&all, "a", false, "show all tasks (shorthand)")
	fs.StringVar(&project, "project", "", "filter by project")
	fs.StringVar(&project, "p", "", "filter by project (shorthand)")
	fs.StringVar(&status, "status", "", "filter by status (open|done|archived)")
	fs.IntVar(&limit, "limit", 0, "limit number of tasks")
	fs.IntVar(&limit, "n", 0, "limit number of tasks (shorthand)")
	fs.StringVar(&tag, "tag", "", "filter by tag")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(ctx.Err)
		fmt.Fprintln(ctx.Err, listUsage(ctx.AppName))
		return 2
	}

	if len(fs.Args()) != 0 {
		fmt.Fprintf(ctx.Err, "Error: unexpected arguments\n")
		fmt.Fprintln(ctx.Err, listUsage(ctx.AppName))
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
		fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	// Ensure open tasks have short_ids (for display)
	for _, t := range tasks {
		if t.Status == task.StatusOpen {
			_ = st.EnsureShortID(t) // Ignore errors, just try to ensure short_ids
		}
	}

	// Reload to get updated tasks with short_ids
	tasks, err = st.LoadAll()
	if err != nil {
		fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	if len(tasks) == 0 {
		fmt.Fprintln(ctx.Out, "No tasks found.")
		return 0
	}

	// Filter tasks
	filtered := filterTasks(tasks, all, status, project, tag)

	if len(filtered) == 0 {
		fmt.Fprintln(ctx.Out, "No tasks found.")
		return 0
	}

	// Apply limit
	if limit > 0 && limit < len(filtered) {
		filtered = filtered[:limit]
	}

	// Display tasks
	displayTasks(ctx.Out, filtered)

	return 0
}

func listUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s list [flags]

Flags:
  --path <dir>                custom workspace path
  -a, --all                   show all tasks (default: only open)
  -p, --project <name>        filter by project
  --status <open|done|archived> filter by status
  -n, --limit <n>             limit number of tasks
  --tag <tag>                 filter by tag (normalized)

`, app)
}

// filterTasks filters tasks based on the provided criteria.
func filterTasks(tasks []*task.Task, all bool, statusFilter, projectFilter, tagFilter string) []*task.Task {
	var filtered []*task.Task

	// Normalize tag filter
	var normalizedTagFilter string
	if tagFilter != "" {
		normalized := task.NormalizeTags([]string{tagFilter})
		if len(normalized) > 0 {
			normalizedTagFilter = normalized[0]
		}
	}

	for _, t := range tasks {
		// Status filter
		if statusFilter != "" {
			if string(t.Status) != statusFilter {
				continue
			}
		} else if !all {
			// Default: only show open tasks
			if t.Status != task.StatusOpen {
				continue
			}
		}

		// Project filter
		if projectFilter != "" && t.Project != projectFilter {
			continue
		}

		// Tag filter (exact match in normalized tags)
		if normalizedTagFilter != "" {
			found := false
			for _, tag := range t.Tags {
				if tag == normalizedTagFilter {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		filtered = append(filtered, t)
	}

	return filtered
}

// displayTasks displays tasks in list format.
func displayTasks(out io.Writer, tasks []*task.Task) {
	flagMap := map[task.Status]string{
		task.StatusOpen:     " ",
		task.StatusDone:     "x",
		task.StatusArchived: "-",
	}

	for _, t := range tasks {
		flag := flagMap[t.Status]
		if flag == "" {
			flag = "?"
		}

		// Format short_id (only for open tasks)
		var sidStr string
		if t.Status == task.StatusOpen && t.ShortID != nil {
			sidStr = fmt.Sprintf("%4d", *t.ShortID)
		} else {
			sidStr = "    "
		}

		// Build line
		line := fmt.Sprintf("%s [%s] %s (%s)", sidStr, flag, t.Title, t.ID)

		// Add project
		if t.Project != "" {
			line += fmt.Sprintf(" (#%s)", t.Project)
		}

		// Add due date
		if t.DueAt != nil {
			line += fmt.Sprintf("  due %s", t.DueAt.Format("2006-01-02"))
		}

		// Add tags
		if len(t.Tags) > 0 {
			tagStrs := make([]string, len(t.Tags))
			for i, tag := range t.Tags {
				tagStrs[i] = "#" + tag
			}
			line += fmt.Sprintf("  [%s]", strings.Join(tagStrs, ","))
		}

		fmt.Fprintln(out, line)
	}
}
