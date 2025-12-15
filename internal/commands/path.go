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
		fmt.Fprintln(ctx.Err, pathUsage(ctx.AppName))
	}

	var path string
	fs.StringVar(&path, "path", "", "custom workspace path")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(ctx.Err)
		fmt.Fprintln(ctx.Err, pathUsage(ctx.AppName))
		return 2
	}

	threadIDs := fs.Args()
	if len(threadIDs) == 0 {
		fmt.Fprintf(ctx.Err, "Error: missing argument: thread ID required\n")
		return 2
	}

	if len(threadIDs) > 1 {
		fmt.Fprintf(ctx.Err, "Error: too many arguments (expected one thread ID)\n")
		return 2
	}

	threadID := threadIDs[0]

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

	// Resolve thread path
	threadPath := store.ThreadPath(paths.ThreadsDir, threadID)

	// Print only the path, followed by a newline (no extra text)
	fmt.Fprintf(ctx.Out, "%s\n", threadPath)

	return 0
}

func pathUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s path [--path <dir>] <thread-id>

Prints the canonical filesystem path for the thread directory.

Flags:
  --path <dir>   custom workspace path

`, app)
}
