package commands

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/sjatkinson/threadkeeper/internal/config"
	"github.com/sjatkinson/threadkeeper/internal/date"
	"github.com/sjatkinson/threadkeeper/internal/store"
	"github.com/sjatkinson/threadkeeper/internal/task"
)

type updateStringList []string

func (s *updateStringList) String() string { return strings.Join(*s, ",") }
func (s *updateStringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}
func (s *updateStringList) Type() string { return "updateStringList" }

func RunUpdate(args []string, ctx CommandContext) int {
	fs := flag.NewFlagSet(ctx.AppName+" update", flag.ContinueOnError)
	fs.SetOutput(ctx.Err)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(ctx.Err, updateUsage(ctx.AppName))
	}

	var (
		path       string
		title      string
		due        string
		project    string
		addTags    updateStringList
		removeTags updateStringList
	)

	fs.StringVar(&path, "path", "", "custom workspace path")
	fs.StringVar(&title, "title", "", "set new title")
	fs.StringVar(&due, "due", "", "set due date (YYYY-MM-DD)")
	fs.StringVar(&project, "project", "", "set project name")
	fs.Var(&addTags, "add-tag", "repeatable tag to add")
	fs.Var(&removeTags, "remove-tag", "repeatable tag to remove")

	// Pre-process args: convert -tag to --remove-tag tag
	// Since we have no short flags, any -X (where X is not --) can be treated as tag removal
	processedArgs := make([]string, 0, len(args))
	for _, arg := range args {
		// Check if this looks like a short flag that should be converted to tag removal
		// Pattern: -X where X is not -- (long flag)
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && len(arg) > 1 {
			// Convert -tag to --remove-tag tag
			tagName := arg[1:] // Remove the leading -
			processedArgs = append(processedArgs, "--remove-tag", tagName)
		} else {
			processedArgs = append(processedArgs, arg)
		}
	}

	if err := fs.Parse(processedArgs); err != nil {
		if err == flag.ErrHelp {
			fs.Usage()
			return 0
		}
		_, _ = fmt.Fprintln(ctx.Err)
		_, _ = fmt.Fprintln(ctx.Err, updateUsage(ctx.AppName))
		return 2
	}

	// Parse positional arguments: separate IDs from +tag shortcuts
	// Note: -tag is already handled by pre-processing above
	remaining := fs.Args()
	var ids []string
	for _, arg := range remaining {
		if strings.HasPrefix(arg, "+") {
			// Add tag shortcut: +tag
			if len(arg) > 1 {
				addTags = append(addTags, arg[1:])
			}
		} else {
			// Regular ID
			ids = append(ids, arg)
		}
	}

	if len(ids) == 0 {
		_, _ = fmt.Fprintf(ctx.Err, "Error: missing argument: task ID required\n")
		return 2
	}

	// Check if at least one update field was provided
	hasAddTags := len(addTags) > 0
	hasRemoveTags := len(removeTags) > 0
	if title == "" && due == "" && project == "" && !hasAddTags && !hasRemoveTags {
		_, _ = fmt.Fprintf(ctx.Err, "Error: nothing to update. Provide --title/--due/--project/--add-tag/--remove-tag or use +tag/-tag shortcuts.\n")
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

	// Normalize tags
	normalizedAddTags := task.NormalizeTags([]string(addTags))
	normalizedRemoveTags := task.NormalizeTags([]string(removeTags))

	// Parse due date if provided
	var dueAt *time.Time
	if due != "" {
		// Load date locale from config
		locale, err := config.LoadDateLocale()
		if err != nil {
			locale = config.DateLocaleISO // Default on error
		}

		// Parse date using locale-aware parser
		canonical, err := date.ParseDate(due, locale, date.RealClock{}, nil)
		if err != nil {
			_, _ = fmt.Fprintf(ctx.Err, "Error: %v\n", err)
			return 1
		}

		// Convert canonical string to time.Time
		parsed, err := time.Parse("2006-01-02", canonical)
		if err != nil {
			_, _ = fmt.Fprintf(ctx.Err, "Error: failed to parse canonical date: %v\n", err)
			return 1
		}
		parsed = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
		dueAt = &parsed
	}

	// Update each task
	now := time.Now().UTC()
	for _, t := range tasks {
		changed := false

		// Update title
		if title != "" && title != t.Title {
			t.Title = title
			changed = true
		}

		// Update due date
		if dueAt != nil {
			// Compare dates (ignore time component)
			taskDueDate := time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC)
			if t.DueAt != nil {
				taskDueDate = *t.DueAt
			}
			newDueDate := time.Date(dueAt.Year(), dueAt.Month(), dueAt.Day(), 0, 0, 0, 0, time.UTC)
			taskDueDate = time.Date(taskDueDate.Year(), taskDueDate.Month(), taskDueDate.Day(), 0, 0, 0, 0, time.UTC)

			if newDueDate != taskDueDate {
				t.DueAt = dueAt
				changed = true
			}
		}

		// Update project
		if project != "" && project != t.Project {
			t.Project = project
			changed = true
		}

		// Update tags
		if hasAddTags || hasRemoveTags {
			existingTags := make(map[string]bool)
			for _, tag := range t.Tags {
				existingTags[tag] = true
			}

			// Make a copy to compare later
			beforeTags := make(map[string]bool)
			for tag := range existingTags {
				beforeTags[tag] = true
			}

			// Add tags
			for _, tag := range normalizedAddTags {
				existingTags[tag] = true
			}

			// Remove tags
			for _, tag := range normalizedRemoveTags {
				delete(existingTags, tag)
			}

			// Check if tags actually changed (compare sets)
			if len(existingTags) != len(beforeTags) {
				changed = true
			} else {
				// Same size, but could be different tags
				for tag := range existingTags {
					if !beforeTags[tag] {
						changed = true
						break
					}
				}
				if !changed {
					for tag := range beforeTags {
						if !existingTags[tag] {
							changed = true
							break
						}
					}
				}
			}

			if changed {
				// Convert map back to sorted slice
				t.Tags = make([]string, 0, len(existingTags))
				for tag := range existingTags {
					t.Tags = append(t.Tags, tag)
				}
				sort.Strings(t.Tags)
			}
		}

		// Save if changed
		if changed {
			t.UpdatedAt = now
			if err := st.Save(t); err != nil {
				_, _ = fmt.Fprintf(ctx.Err, "Error: failed to save task %s: %v\n", t.ID, err)
				return 1
			}

			// Print confirmation
			sidStr := "?"
			if t.ShortID != nil {
				sidStr = fmt.Sprintf("%d", *t.ShortID)
			}
			_, _ = fmt.Fprintf(ctx.Out, "Updated task %s (%s)\n", sidStr, t.ID)
		}
	}

	return 0
}

func updateUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s update [--path <dir>] [flags] <id> [<id> ...] [+tag] [-tag] ...

Flags:
  --path <dir>        custom workspace path
  --title <string>    set new title
  --due <date>        set due date (format depends on date_locale config)
  --project <name>    set project name
  --add-tag <tag>     add a tag (repeatable)
  --remove-tag <tag>  remove a tag (repeatable)

Tag shortcuts:
  +tag                add a tag (e.g., +foo)
  -tag                remove a tag (e.g., -bar)

Due date shortcuts:
  today               set due date to today
  +N                  set due date to today + N days (e.g., +1, +2, +7)

Examples:
  %s update 3 +foo -bar
  %s update 3 --title "New title" +important
  %s update 3 --due today
  %s update 3 --due +7

`, app, app, app, app, app)
}
