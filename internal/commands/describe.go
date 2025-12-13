package commands

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sjatkinson/threadkeeper/internal/config"
	"github.com/sjatkinson/threadkeeper/internal/store"
)

func RunDescribe(args []string, ctx CommandContext) int {
	fs := flag.NewFlagSet(ctx.AppName+" describe", flag.ContinueOnError)
	fs.SetOutput(ctx.Err)
	fs.Usage = func() {
		fmt.Fprintln(ctx.Err, describeUsage(ctx.AppName))
	}

	var path string
	fs.StringVar(&path, "path", "", "custom workspace path")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(ctx.Err)
		fmt.Fprintln(ctx.Err, describeUsage(ctx.AppName))
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

	if _, err := os.Stat(paths.TasksDir); err != nil {
		fmt.Fprintf(ctx.Err, "Error: tasks directory does not exist at %s. Run '%s init' first.\n", paths.TasksDir, ctx.AppName)
		return 1
	}

	// Load and resolve task
	st := store.NewFileStore(paths.TasksDir)
	t, err := st.ResolveID(idStr)
	if err != nil {
		fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	// Get current description
	currentDesc := t.Description

	// Get editor
	editor := getEditor()

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "tk-describe-*.txt")
	if err != nil {
		fmt.Fprintf(ctx.Err, "Error: failed to create temporary file: %v\n", err)
		return 1
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up temp file

	// Write current description to temp file
	if currentDesc != "" {
		if _, err := tmpFile.WriteString(currentDesc); err != nil {
			fmt.Fprintf(ctx.Err, "Error: failed to write to temporary file: %v\n", err)
			return 1
		}
	}
	if err := tmpFile.Close(); err != nil {
		fmt.Fprintf(ctx.Err, "Error: failed to close temporary file: %v\n", err)
		return 1
	}

	// Launch editor
	// Split editor command to handle cases like "code --wait" or "vim -f"
	editorParts := strings.Fields(editor)
	if len(editorParts) == 0 {
		editorParts = []string{"vi"}
	}

	// Append the temp file path as the last argument
	cmd := exec.Command(editorParts[0], append(editorParts[1:], tmpPath)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(ctx.Err, "Error: editor exited with code %d; description unchanged.\n", exitErr.ExitCode())
			return 1
		}
		fmt.Fprintf(ctx.Err, "Error: failed to run editor: %v\n", err)
		return 1
	}

	// Read edited content
	newText, err := os.ReadFile(tmpPath)
	if err != nil {
		fmt.Fprintf(ctx.Err, "Error: failed to read edited file: %v\n", err)
		return 1
	}

	newTextStr := string(newText)
	newTextStripped := strings.TrimSpace(newTextStr)

	// If empty after stripping, leave description unchanged
	if newTextStripped == "" {
		fmt.Fprintln(ctx.Out, "Empty description; leaving existing description unchanged.")
		return 0
	}

	// Update task description (preserve trailing newlines, but strip trailing whitespace from each line)
	t.Description = strings.TrimRight(newTextStr, " \t\n\r")
	t.UpdatedAt = time.Now().UTC()

	// Save task
	if err := st.Save(t); err != nil {
		fmt.Fprintf(ctx.Err, "Error: failed to save task: %v\n", err)
		return 1
	}

	// Output success message
	sidStr := "?"
	if t.ShortID != nil {
		sidStr = fmt.Sprintf("%d", *t.ShortID)
	}
	fmt.Fprintf(ctx.Out, "Updated description for task %s (%s)\n", sidStr, t.ID)

	return 0
}

func describeUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s describe [--path <dir>] <id>

Flags:
  --path <dir>   custom workspace path

`, app)
}

// getEditor returns the editor command to use, checking EDITOR, VISUAL, or defaulting to "vi".
func getEditor() string {
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}
	return "vi"
}
