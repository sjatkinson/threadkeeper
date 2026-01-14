package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sjatkinson/threadkeeper/internal/commands"
	"github.com/sjatkinson/threadkeeper/internal/config"
)

// CommandInfo holds metadata for a command.
// This provides a single source of truth for command name, description,
// usage text, and runner function.
type CommandInfo struct {
	Name        string
	Description string
	Usage       func(app string) string
	Runner      func(args []string, ctx commands.CommandContext) int
}

// commandRegistry holds all registered commands.
// Commands are registered in init() and provide a single source of truth
// for command metadata, dispatch, and help generation.
var commandRegistry = make(map[string]CommandInfo)

// registerCommand adds a command to the registry.
// This is idempotent - registering the same command twice overwrites the first.
func registerCommand(info CommandInfo) {
	commandRegistry[info.Name] = info
}

// getCommand returns the CommandInfo for the given command name,
// or nil if the command is not registered.
func getCommand(name string) *CommandInfo {
	info, ok := commandRegistry[name]
	if !ok {
		return nil
	}
	return &info
}

// getAllCommands returns all registered commands sorted by name.
// This is used for generating the usage output.
func getAllCommands() []CommandInfo {
	cmds := make([]CommandInfo, 0, len(commandRegistry))
	for _, cmd := range commandRegistry {
		cmds = append(cmds, cmd)
	}
	// Sort by name for consistent output
	for i := 0; i < len(cmds)-1; i++ {
		for j := i + 1; j < len(cmds); j++ {
			if cmds[i].Name > cmds[j].Name {
				cmds[i], cmds[j] = cmds[j], cmds[i]
			}
		}
	}
	return cmds
}

func init() {
	// Register all commands with their metadata
	registerCommand(CommandInfo{
		Name:        "init",
		Description: "Initialize the workspace",
		Usage:       initUsage,
		Runner:      commands.RunInit,
	})
	registerCommand(CommandInfo{
		Name:        "add",
		Description: "Add a new task",
		Usage:       addUsage,
		Runner:      commands.RunAdd,
	})
	registerCommand(CommandInfo{
		Name:        "list",
		Description: "List tasks",
		Usage:       listUsage,
		Runner:      commands.RunList,
	})
	registerCommand(CommandInfo{
		Name:        "show",
		Description: "Show details for a single task",
		Usage:       showUsage,
		Runner:      commands.RunShow,
	})
	registerCommand(CommandInfo{
		Name:        "describe",
		Description: "Edit a task description in $EDITOR (later)",
		Usage:       describeUsage,
		Runner:      commands.RunDescribe,
	})
	registerCommand(CommandInfo{
		Name:        "update",
		Description: "Update fields on one or more tasks",
		Usage:       updateUsage,
		Runner:      commands.RunUpdate,
	})
	registerCommand(CommandInfo{
		Name:        "done",
		Description: "Mark one or more tasks done",
		Usage:       doneUsage,
		Runner:      commands.RunDone,
	})
	registerCommand(CommandInfo{
		Name:        "archive",
		Description: "Archive one or more tasks",
		Usage:       archiveUsage,
		Runner:      commands.RunArchive,
	})
	registerCommand(CommandInfo{
		Name:        "reopen",
		Description: "Reopen one or more tasks (change from inactive to active)",
		Usage:       reopenUsage,
		Runner:      commands.RunReopen,
	})
	registerCommand(CommandInfo{
		Name:        "remove",
		Description: "Remove one or more tasks (hard delete; requires --force)",
		Usage:       removeUsage,
		Runner:      commands.RunRemove,
	})
	registerCommand(CommandInfo{
		Name:        "reindex",
		Description: "Reassign short IDs for active tasks",
		Usage:       reindexUsage,
		Runner:      commands.RunReindex,
	})
	registerCommand(CommandInfo{
		Name:        "path",
		Description: "Print filesystem path for a thread directory",
		Usage:       pathUsage,
		Runner:      commands.RunPath,
	})
	registerCommand(CommandInfo{
		Name:        "attach",
		Description: "Attach an inline note to a thread",
		Usage:       attachUsage,
		Runner:      commands.RunAttach,
	})
	registerCommand(CommandInfo{
		Name:        "open",
		Description: "Open an attachment from a thread",
		Usage:       openUsage,
		Runner:      commands.RunOpen,
	})
}

type Config struct {
	AppName string
	Out     io.Writer
	Err     io.Writer

	Version string

	Verbose bool
	Debug   bool
}

func Run(argv []string, cfg Config) int {
	if cfg.Out == nil {
		cfg.Out = os.Stdout
	}
	if cfg.Err == nil {
		cfg.Err = os.Stderr
	}
	if cfg.AppName == "" {
		cfg.AppName = "tk"
	}
	if cfg.Version == "" {
		cfg.Version = "0.0.0-dev"
	}

	// ---- Global flags ----
	global := flag.NewFlagSet(cfg.AppName, flag.ContinueOnError)
	global.SetOutput(cfg.Err)

	var (
		flgHelp    bool
		flgVersion bool
		flgPath    string
	)
	global.BoolVar(&flgHelp, "h", false, "show help")
	global.BoolVar(&flgHelp, "help", false, "show help")
	global.BoolVar(&flgVersion, "version", false, "print version and exit")
	global.BoolVar(&cfg.Verbose, "v", false, "verbose output")
	global.BoolVar(&cfg.Verbose, "verbose", false, "verbose output")
	global.BoolVar(&cfg.Debug, "debug", false, "debug output")
	global.StringVar(&flgPath, "path", "", "custom workspace path")

	global.Usage = func() { _, _ = fmt.Fprintln(cfg.Err, usage(cfg.AppName)) }

	if err := global.Parse(argv); err != nil {
		_, _ = fmt.Fprintln(cfg.Err)
		_, _ = fmt.Fprintln(cfg.Err, usage(cfg.AppName))
		return 2
	}

	if flgVersion {
		_, _ = fmt.Fprintf(cfg.Out, "%s %s\n", cfg.AppName, cfg.Version)
		return 0
	}

	rest := global.Args()
	if flgHelp {
		_, _ = fmt.Fprintln(cfg.Err, usage(cfg.AppName))
		return 0
	}

	// If no command provided, check if workspace exists
	// If it exists, default to 'list'. Otherwise show usage.
	if len(rest) == 0 {
		paths, err := config.GetPaths("")
		if err == nil {
			// Check if threads directory exists
			if _, err := os.Stat(paths.ThreadsDir); err == nil {
				// Workspace exists, run list command
				return commands.RunList([]string{}, commands.CommandContext{
					AppName: cfg.AppName,
					Out:     cfg.Out,
					Err:     cfg.Err,
					Path:    flgPath,
				})
			}
		}
		// No workspace exists, show usage
		_, _ = fmt.Fprintln(cfg.Err, usage(cfg.AppName))
		return 0
	}

	cmd := rest[0]
	args := rest[1:]

	// Load aliases from config
	rawAliases, err := config.LoadAliases()
	if err != nil {
		// Log warning but continue (don't fail on malformed config)
		if cfg.Verbose || cfg.Debug {
			_, _ = fmt.Fprintf(cfg.Err, "Warning: failed to load aliases: %v\n", err)
		}
		rawAliases = make(config.Aliases)
	}

	// Validate and filter aliases
	aliases := validateAliases(rawAliases, cfg.Verbose || cfg.Debug, cfg.Err)

	// Resolve alias: built-in commands take precedence
	if getCommand(cmd) == nil {
		if target, ok := aliases[cmd]; ok {
			// Alias is already validated, so target is guaranteed to be a built-in
			cmd = target
		}
	}

	// Handle special case: help command
	if cmd == "help" {
		if len(args) == 0 {
			_, _ = fmt.Fprintln(cfg.Err, usage(cfg.AppName))
			return 0
		}
		_, _ = fmt.Fprintln(cfg.Err, commandUsage(cfg.AppName, args[0]))
		return 0
	}

	// Look up command in registry and dispatch
	info := getCommand(cmd)
	if info == nil {
		_, _ = fmt.Fprintf(cfg.Err, "unknown command: %q\n\n", cmd)
		_, _ = fmt.Fprintln(cfg.Err, usage(cfg.AppName))
		return 2
	}

	return info.Runner(args, commands.CommandContext{
		AppName: cfg.AppName,
		Out:     cfg.Out,
		Err:     cfg.Err,
		Path:    flgPath,
	})
}

func usage(app string) string {
	cmds := getAllCommands()

	// Preserve specific ordering: init first, help last, others in registration order
	// Build ordered list manually to maintain desired output
	orderedNames := []string{"init", "add", "list", "show", "describe", "update", "done", "archive", "reopen", "remove", "reindex", "path", "attach", "open"}

	var cmdLines []string
	seen := make(map[string]bool)

	// Add commands in desired order
	for _, name := range orderedNames {
		if info := getCommand(name); info != nil {
			cmdLines = append(cmdLines, fmt.Sprintf("  %-10s  %s", info.Name, info.Description))
			seen[name] = true
		}
	}

	// Add any remaining commands (shouldn't happen, but be safe)
	for _, cmd := range cmds {
		if !seen[cmd.Name] {
			cmdLines = append(cmdLines, fmt.Sprintf("  %-10s  %s", cmd.Name, cmd.Description))
		}
	}

	// Add help command (special case, not in registry)
	cmdLines = append(cmdLines, "  help      Help for a command")

	return fmt.Sprintf(`%s: a local-first task tracker

Usage:
  %s [global flags] <command> [command flags] [args]

Global flags:
  -h, --help           show help
      --version        print version and exit
  -v, --verbose        verbose output
      --debug          debug output
      --path <dir>     custom workspace path

Commands:
%s

Run:
  %s help <command>
`, app, app, strings.Join(cmdLines, "\n"), app)
}

// Usage functions extracted from commandUsage() switch
func initUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s init [--force]

Flags:
  --force          allow initialization even if tasks exist (future: may wipe)

`, app)
}

func addUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s add <title> [flags]

Flags:
  -d, --description <t>  description
  -p, --project <name>   project name
  --due <date>           due date (format depends on date_locale config)
  --tag <tag>            repeatable

`, app)
}

func listUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s list [flags]

Flags:
  -a, --all                   show all tasks (default: only open)
  -p, --project <name>        filter by project
  --status <open|done|archived> filter by status
  -n, --limit <n>             limit number of tasks
  --tag <tag>                 filter by tag (normalized)

`, app)
}

func doneUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s done <id> [<id> ...]

`, app)
}

func removeUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s remove --force <id> [<id> ...]

Flags:
  --force   actually delete (required)

`, app)
}

func archiveUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s archive <id> [<id> ...]

`, app)
}

func reopenUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s reopen <id> [<id> ...]

Reopen one or more tasks, changing their status from inactive (archived or done) to active.

`, app)
}

func reindexUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s reindex

`, app)
}

func describeUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s describe <id>

`, app)
}

func showUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s show [--full] <id>

Flags:
  --full         show full metadata and history
  --all          show full metadata (deprecated, use --full)

`, app)
}

func updateUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s update <id> [<id> ...] [flags]

Flags:
  --title <t>           set new title
  --due <date>          set due date (format depends on date_locale config)
  --project <name>      set project name
  --add-tag <tag>       repeatable
  --remove-tag <tag>    repeatable

`, app)
}

func pathUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s path <thread-id>

Prints the canonical filesystem path for the thread directory.
Accepts either a durable thread ID or a short ID.

`, app)
}

func attachUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s attach note --id <thread-id>
  %s attach link --id <thread-id> --url <url> [--label <label>]

Attach context to a thread.

Types:
  note   Open editor, store content-addressed blob, record in attachments.jsonl.
  link   Record URL (and optional label) in attachments.jsonl.

Flags:
  --id <id>       thread handle or canonical id
  --url <url>     URL to attach [link only]
  --label <text>  label for link (pr, slack, jira, doc, etc.) [link only]

Environment variables:
  TK_EDITOR       editor to use (defaults to $EDITOR, then vi) [note only]
  EDITOR          editor to use (if TK_EDITOR not set) [note only]

Examples:
  %s attach note --id 1
  %s attach link --id 1 --url https://example.com/pr/123 --label pr

`, app, app, app, app)
}

func openUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s open [--att <index> | --att-id <id>] [--print-path] <thread-id>

Open an attachment from a thread.

Flags:
  --att <index>     attachment index (1-based, from 'show' output)
  --att-id <id>     attachment ID (alternative to --att)
  --print-path      print blob path instead of opening

Examples:
  %s open 1 --att 1
  %s open 1 --att-id 01ARZ3NDEKTSV4RRFFQ69G5FAV --print-path

`, app, app, app)
}

func commandUsage(app, cmd string) string {
	info := getCommand(cmd)
	if info == nil {
		return fmt.Sprintf("Unknown command %q\n\n%s", cmd, usage(app))
	}
	return info.Usage(app)
}

// validateAliases filters and validates aliases:
// - Removes aliases that conflict with built-in commands (built-in wins)
// - Removes aliases that point to non-existent commands
// - Removes aliases that point to other aliases (no recursion)
// Returns a validated map of alias -> built-in command.
func validateAliases(raw config.Aliases, verbose bool, errOut io.Writer) config.Aliases {
	valid := make(config.Aliases)

	for alias, target := range raw {
		// Skip aliases that conflict with built-in commands
		if getCommand(alias) != nil {
			if verbose {
				_, _ = fmt.Fprintf(errOut, "Warning: alias %q conflicts with built-in command, ignoring\n", alias)
			}
			continue
		}

		// Check if target is a built-in command
		if getCommand(target) == nil {
			// Check if target is another alias (recursion)
			if _, isAlias := raw[target]; isAlias {
				if verbose {
					_, _ = fmt.Fprintf(errOut, "Warning: alias %q points to another alias %q (recursion not allowed), ignoring\n", alias, target)
				}
				continue
			}
			// Target is not a built-in and not an alias - invalid
			if verbose {
				_, _ = fmt.Fprintf(errOut, "Warning: alias %q points to non-existent command %q, ignoring\n", alias, target)
			}
			continue
		}

		// Valid alias: points directly to a built-in command
		valid[alias] = target
	}

	return valid
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}
