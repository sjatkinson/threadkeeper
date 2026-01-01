package commands

import (
	"flag"
	"fmt"
	"os"

	"github.com/sjatkinson/threadkeeper/internal/config"
	"github.com/sjatkinson/threadkeeper/internal/store"
)

func RunPath(args []string, ctx CommandContext) int {
	fs := flag.NewFlagSet(ctx.AppName+" path", flag.ContinueOnError)
	fs.SetOutput(ctx.Err)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(ctx.Err, pathUsage(ctx.AppName))
	}

	var path string
	fs.StringVar(&path, "path", "", "custom workspace path")

	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintln(ctx.Err)
		_, _ = fmt.Fprintln(ctx.Err, pathUsage(ctx.AppName))
		return 2
	}

	threadIDs := fs.Args()
	if len(threadIDs) == 0 {
		_, _ = fmt.Fprintf(ctx.Err, "Error: missing argument: thread ID required\n")
		return 2
	}

	if len(threadIDs) > 1 {
		_, _ = fmt.Fprintf(ctx.Err, "Error: too many arguments (expected one thread ID)\n")
		return 2
	}

	threadID := threadIDs[0]

	// Get paths and verify threads directory exists
	paths, err := config.GetPaths(path)
	if err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	if _, err := os.Stat(paths.ThreadsDir); err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: threads directory does not exist at %s. Run '%s init' first.\n", paths.ThreadsDir, ctx.AppName)
		return 1
	}

	// Resolve ID (handles both durable IDs and short IDs)
	st := store.NewFileStore(paths.ThreadsDir)
	t, err := st.ResolveID(threadID)
	if err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	// Resolve thread path using the durable ID
	threadPath := store.ThreadPath(paths.ThreadsDir, t.ID)

	// Print only the path, followed by a newline (no extra text)
	_, _ = fmt.Fprintf(ctx.Out, "%s\n", threadPath)

	return 0
}

func pathUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s path [--path <dir>] <thread-id>

Prints the canonical filesystem path for the thread directory.
Accepts either a durable thread ID or a short ID.

Flags:
  --path <dir>   custom workspace path

`, app)
}
