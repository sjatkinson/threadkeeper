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
}

func RunInit(args []string, ctx CommandContext) int {
	fs := flag.NewFlagSet(ctx.AppName+" init", flag.ContinueOnError)
	fs.SetOutput(ctx.Err)
	fs.Usage = func() {
		fmt.Fprintln(ctx.Err, usage(ctx.AppName))
	}

	var path string
	var force bool
	fs.StringVar(&path, "path", "", "custom workspace path")
	fs.BoolVar(&force, "force", false, "force initialization (wipes threads directory)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(ctx.Err)
		fmt.Fprintln(ctx.Err, usage(ctx.AppName))
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(ctx.Err, usage(ctx.AppName))
		return 2
	}

	paths, err := config.GetPaths(path)
	if err != nil {
		fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	// Ensure workspace exists
	if err := os.MkdirAll(paths.Workspace, 0o755); err != nil {
		fmt.Fprintf(ctx.Err, "Error: failed to create workspace directory: %v\n", err)
		return 1
	}

	// Threads dir handling
	existed := dirExists(paths.ThreadsDir)

	if existed {
		// Threads directory already exists
		if force {
			// --force was specified: delete entire threads directory
			if err := os.RemoveAll(paths.ThreadsDir); err != nil {
				fmt.Fprintf(ctx.Err, "Error: failed to delete threads directory: %v\n", err)
				return 1
			}
			// Recreate the directory
			if err := os.MkdirAll(paths.ThreadsDir, 0o755); err != nil {
				fmt.Fprintf(ctx.Err, "Error: failed to create threads directory: %v\n", err)
				return 1
			}
			fmt.Fprintf(ctx.Out, "Initialized workspace: %s\n", paths.Workspace)
			fmt.Fprintf(ctx.Out, "Threads directory    : %s\n", paths.ThreadsDir)
			fmt.Fprintln(ctx.Out, "Note: --force was used; threads directory was removed and recreated.")
			return 0
		}
		// No --force: show warning and don't touch anything
		fmt.Fprintf(ctx.Err, "Warning: threads directory %s already exists (use --force to reinitialize)\n", paths.ThreadsDir)
		fmt.Fprintf(ctx.Out, "Initialized workspace: %s\n", paths.Workspace)
		fmt.Fprintf(ctx.Out, "Threads directory    : %s\n", paths.ThreadsDir)
		return 0
	}

	// Threads dir doesn't exist - create it
	if err := os.MkdirAll(paths.ThreadsDir, 0o755); err != nil {
		fmt.Fprintf(ctx.Err, "Error: failed to create threads directory: %v\n", err)
		return 1
	}

	fmt.Fprintf(ctx.Out, "Initialized workspace: %s\n", paths.Workspace)
	fmt.Fprintf(ctx.Out, "Threads directory    : %s\n", paths.ThreadsDir)
	return 0
}

func usage(app string) string {
	return fmt.Sprintf(`Usage:
  %s init [--path <dir>] [--force]

Flags:
  --path <dir>     custom workspace path
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
