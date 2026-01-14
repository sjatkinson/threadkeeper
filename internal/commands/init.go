package commands

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sjatkinson/threadkeeper/internal/config"
)

// CommandContext provides the context needed for command execution.
// This avoids import cycles between cli and commands packages.
type CommandContext struct {
	AppName string
	Out     io.Writer
	Err     io.Writer
	Path    string
}

func RunInit(args []string, ctx CommandContext) int {
	fs := flag.NewFlagSet(ctx.AppName+" init", flag.ContinueOnError)
	fs.SetOutput(ctx.Err)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(ctx.Err, usage(ctx.AppName))
	}

	var force bool
	fs.BoolVar(&force, "force", false, "force initialization (wipes threads directory)")

	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintln(ctx.Err)
		_, _ = fmt.Fprintln(ctx.Err, usage(ctx.AppName))
		return 2
	}
	if len(fs.Args()) != 0 {
		_, _ = fmt.Fprintln(ctx.Err, usage(ctx.AppName))
		return 2
	}

	paths, err := config.GetPaths(ctx.Path)
	if err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	// Ensure workspace exists
	if err := os.MkdirAll(paths.Workspace, 0o755); err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: failed to create workspace directory: %v\n", err)
		return 1
	}

	// Threads dir handling
	existed := dirExists(paths.ThreadsDir)

	if existed {
		// Threads directory already exists
		if force {
			// --force was specified: delete entire threads directory
			if err := os.RemoveAll(paths.ThreadsDir); err != nil {
				_, _ = fmt.Fprintf(ctx.Err, "Error: failed to delete threads directory: %v\n", err)
				return 1
			}
			// Recreate the directory
			if err := os.MkdirAll(paths.ThreadsDir, 0o755); err != nil {
				_, _ = fmt.Fprintf(ctx.Err, "Error: failed to create threads directory: %v\n", err)
				return 1
			}
			_, _ = fmt.Fprintf(ctx.Out, "Initialized workspace: %s\n", paths.Workspace)
			_, _ = fmt.Fprintf(ctx.Out, "Threads directory    : %s\n", paths.ThreadsDir)
			_, _ = fmt.Fprintln(ctx.Out, "Note: --force was used; threads directory was removed and recreated.")
			return 0
		}
		// No --force: show warning and don't touch anything
		_, _ = fmt.Fprintf(ctx.Err, "Warning: threads directory %s already exists (use --force to reinitialize)\n", paths.ThreadsDir)
		_, _ = fmt.Fprintf(ctx.Out, "Initialized workspace: %s\n", paths.Workspace)
		_, _ = fmt.Fprintf(ctx.Out, "Threads directory    : %s\n", paths.ThreadsDir)
		return 0
	}

	// Threads dir doesn't exist - create it
	if err := os.MkdirAll(paths.ThreadsDir, 0o755); err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: failed to create threads directory: %v\n", err)
		return 1
	}

	_, _ = fmt.Fprintf(ctx.Out, "Initialized workspace: %s\n", paths.Workspace)
	_, _ = fmt.Fprintf(ctx.Out, "Threads directory    : %s\n", paths.ThreadsDir)
	return 0
}

func usage(app string) string {
	return fmt.Sprintf(`Usage:
  %s init [--force]

Flags:
  --force          allow initialization even if tasks exist (future: may wipe)

`, app)
}

// dirExists returns true if the path exists and is a directory.
func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// dirHasRegularFiles returns true if the directory contains any regular files.
func dirHasRegularFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Type().IsRegular() {
			return true
		}
	}
	return false
}

// deleteRegularFiles deletes all regular files in the given directory.
func deleteRegularFiles(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Type().IsRegular() {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
