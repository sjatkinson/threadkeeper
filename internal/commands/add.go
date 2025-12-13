package commands

import (
	"fmt"
	"os"
	"strings"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/sjatkinson/threadkeeper/internal/config"
	"github.com/sjatkinson/threadkeeper/internal/store"
	"github.com/sjatkinson/threadkeeper/internal/task"
)

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}
func (s *stringList) Type() string { return "stringList" }

func RunAdd(args []string, ctx CommandContext) int {
	fs := flag.NewFlagSet(ctx.AppName+" add", flag.ContinueOnError)
	fs.SetOutput(ctx.Err)
	fs.Usage = func() {
		fmt.Fprintln(ctx.Err, addUsage(ctx.AppName))
	}

	var (
		path    string
		desc    string
		project string
		due     string
		tags    stringList
	)
	fs.StringVar(&path, "path", "", "custom workspace path")
	fs.StringVar(&desc, "description", "", "description")
	fs.StringVar(&desc, "d", "", "description (shorthand)")
	fs.StringVar(&project, "project", "", "project name")
	fs.StringVar(&project, "p", "", "project name (shorthand)")
	fs.StringVar(&due, "due", "", "due date (YYYY-MM-DD)")
	fs.Var(&tags, "tag", "repeatable tag")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(ctx.Err)
		fmt.Fprintln(ctx.Err, addUsage(ctx.AppName))
		return 2
	}

	if len(fs.Args()) == 0 {
		fmt.Fprintf(ctx.Err, "Error: missing argument: title required\n")
		return 2
	}

	title := strings.Join(fs.Args(), " ")

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

	// Generate task ID
	taskID, err := task.GenerateID()
	if err != nil {
		fmt.Fprintf(ctx.Err, "Error: failed to generate task ID: %v\n", err)
		return 1
	}

	// Parse due date if provided
	var dueAt *time.Time
	if due != "" {
		parsed, err := time.Parse("2006-01-02", due)
		if err != nil {
			fmt.Fprintf(ctx.Err, "Error: invalid due date format (expected YYYY-MM-DD): %v\n", err)
			return 1
		}
		parsed = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
		dueAt = &parsed
	}

	// Normalize tags
	normalizedTags := task.NormalizeTags([]string(tags))

	// Get next short_id
	st := store.NewFileStore(paths.TasksDir)
	shortID, err := st.GenerateNextShortID()
	if err != nil {
		fmt.Fprintf(ctx.Err, "Error: failed to generate short_id: %v\n", err)
		return 1
	}

	// Create task
	now := time.Now().UTC()
	t := &task.Task{
		ID:          taskID,
		Title:       title,
		Description: desc,
		Status:      task.StatusOpen,
		CreatedAt:   now,
		UpdatedAt:   now,
		DueAt:       dueAt,
		Project:     project,
		Tags:        normalizedTags,
		ShortID:     &shortID,
	}

	// Save task
	if err := st.Save(t); err != nil {
		fmt.Fprintf(ctx.Err, "Error: failed to save task: %v\n", err)
		return 1
	}

	// Output success message
	fmt.Fprintf(ctx.Out, "Added task %d (%s): %s\n", shortID, taskID, title)

	return 0
}

func addUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s add <title> [flags]

Flags:
  --path <dir>           custom workspace path
  -d, --description <t>  description
  -p, --project <name>   project name
  --due <YYYY-MM-DD>     due date
  --tag <tag>            repeatable tag

`, app)
}
