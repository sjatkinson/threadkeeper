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
	)
	global.BoolVar(&flgHelp, "h", false, "show help")
	global.BoolVar(&flgHelp, "help", false, "show help")
	global.BoolVar(&flgVersion, "version", false, "print version and exit")
	global.BoolVar(&cfg.Verbose, "v", false, "verbose output")
	global.BoolVar(&cfg.Verbose, "verbose", false, "verbose output")
	global.BoolVar(&cfg.Debug, "debug", false, "debug output")

	global.Usage = func() { fmt.Fprintln(cfg.Err, usage(cfg.AppName)) }

	if err := global.Parse(argv); err != nil {
		fmt.Fprintln(cfg.Err)
		fmt.Fprintln(cfg.Err, usage(cfg.AppName))
		return 2
	}

	if flgVersion {
		fmt.Fprintf(cfg.Out, "%s %s\n", cfg.AppName, cfg.Version)
		return 0
	}

	rest := global.Args()
	if flgHelp {
		fmt.Fprintln(cfg.Err, usage(cfg.AppName))
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
				})
			}
		}
		// No workspace exists, show usage
		fmt.Fprintln(cfg.Err, usage(cfg.AppName))
		return 0
	}

	cmd := rest[0]
	args := rest[1:]

	// Define built-in commands (these take precedence over aliases)
	builtInCommands := map[string]bool{
		"help":     true,
		"init":     true,
		"add":      true,
		"list":     true,
		"done":     true,
		"remove":   true,
		"archive":  true,
		"reopen":   true,
		"reindex":  true,
		"describe": true,
		"show":     true,
		"update":   true,
		"path":     true,
		"attach":   true,
	}

	// Load aliases from config
	rawAliases, err := config.LoadAliases()
	if err != nil {
		// Log warning but continue (don't fail on malformed config)
		if cfg.Verbose || cfg.Debug {
			fmt.Fprintf(cfg.Err, "Warning: failed to load aliases: %v\n", err)
		}
		rawAliases = make(config.Aliases)
	}

	// Validate and filter aliases
	aliases := validateAliases(rawAliases, builtInCommands, cfg.Verbose || cfg.Debug, cfg.Err)

	// Resolve alias: built-in commands take precedence
	if !builtInCommands[cmd] {
		if target, ok := aliases[cmd]; ok {
			// Alias is already validated, so target is guaranteed to be a built-in
			cmd = target
		}
	}

	switch cmd {
	case "help":
		if len(args) == 0 {
			fmt.Fprintln(cfg.Err, usage(cfg.AppName))
			return 0
		}
		fmt.Fprintln(cfg.Err, commandUsage(cfg.AppName, args[0]))
		return 0

	case "init":
		return commands.RunInit(args, commands.CommandContext{
			AppName: cfg.AppName,
			Out:     cfg.Out,
			Err:     cfg.Err,
		})
	case "add":
		return commands.RunAdd(args, commands.CommandContext{
			AppName: cfg.AppName,
			Out:     cfg.Out,
			Err:     cfg.Err,
		})
	case "list":
		return commands.RunList(args, commands.CommandContext{
			AppName: cfg.AppName,
			Out:     cfg.Out,
			Err:     cfg.Err,
		})
	case "done":
		return commands.RunDone(args, commands.CommandContext{
			AppName: cfg.AppName,
			Out:     cfg.Out,
			Err:     cfg.Err,
		})
	case "remove":
		return commands.RunRemove(args, commands.CommandContext{
			AppName: cfg.AppName,
			Out:     cfg.Out,
			Err:     cfg.Err,
		})
	case "archive":
		return commands.RunArchive(args, commands.CommandContext{
			AppName: cfg.AppName,
			Out:     cfg.Out,
			Err:     cfg.Err,
		})
	case "reopen":
		return commands.RunReopen(args, commands.CommandContext{
			AppName: cfg.AppName,
			Out:     cfg.Out,
			Err:     cfg.Err,
		})
	case "reindex":
		return commands.RunReindex(args, commands.CommandContext{
			AppName: cfg.AppName,
			Out:     cfg.Out,
			Err:     cfg.Err,
		})
	case "describe":
		return commands.RunDescribe(args, commands.CommandContext{
			AppName: cfg.AppName,
			Out:     cfg.Out,
			Err:     cfg.Err,
		})
	case "show":
		return commands.RunShow(args, commands.CommandContext{
			AppName: cfg.AppName,
			Out:     cfg.Out,
			Err:     cfg.Err,
		})
	case "update":
		return commands.RunUpdate(args, commands.CommandContext{
			AppName: cfg.AppName,
			Out:     cfg.Out,
			Err:     cfg.Err,
		})
	case "path":
		return commands.RunPath(args, commands.CommandContext{
			AppName: cfg.AppName,
			Out:     cfg.Out,
			Err:     cfg.Err,
		})
	case "attach":
		return commands.RunAttach(args, commands.CommandContext{
			AppName: cfg.AppName,
			Out:     cfg.Out,
			Err:     cfg.Err,
		})

	default:
		fmt.Fprintf(cfg.Err, "unknown command: %q\n\n", cmd)
		fmt.Fprintln(cfg.Err, usage(cfg.AppName))
		return 2
	}
}

func usage(app string) string {
	return fmt.Sprintf(`%s: a local-first task tracker

Usage:
  %s [global flags] <command> [command flags] [args]

Global flags:
  -h, --help           show help
      --version        print version and exit
  -v, --verbose        verbose output
      --debug          debug output

Commands:
  init      Initialize the workspace
  add       Add a new task
  list      List tasks
  show      Show details for a single task
  describe  Edit a task description in $EDITOR (later)
  update    Update fields on one or more tasks
  done      Mark one or more tasks done
  archive   Archive one or more tasks
  reopen    Reopen one or more tasks (change from inactive to active)
  remove    Remove one or more tasks (hard delete; requires --force)

  reindex   Reassign short IDs for active tasks
  path      Print filesystem path for a thread directory
  attach    Attach an inline note to a thread
  help      Help for a command

Run:
  %s help <command>
`, app, app, app)
}

func commandUsage(app, cmd string) string {
	switch cmd {
	case "init":
		return fmt.Sprintf(`Usage:
  %s init [--path <dir>] [--force]

Flags:
  --path <dir>     custom workspace path
  --force          allow initialization even if tasks exist (future: may wipe)

`, app)

	case "add":
		return fmt.Sprintf(`Usage:
  %s add <title> [flags]

Flags:
  --path <dir>           custom workspace path
  -d, --description <t>  description
  -p, --project <name>   project name
  --due <date>           due date (format depends on date_locale config)
  --tag <tag>            repeatable

`, app)

	case "list":
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

	case "done":
		return fmt.Sprintf(`Usage:
  %s done [--path <dir>] <id> [<id> ...]

`, app)

	case "remove":
		return fmt.Sprintf(`Usage:
  %s remove [--path <dir>] --force <id> [<id> ...]

Flags:
  --force   actually delete (required)

`, app)

	case "archive":
		return fmt.Sprintf(`Usage:
  %s archive [--path <dir>] <id> [<id> ...]

Flags:
  --path <dir>   custom workspace path

`, app)

	case "reopen":
		return fmt.Sprintf(`Usage:
  %s reopen <id> [<id> ...]

Reopen one or more tasks, changing their status from inactive (archived or done) to active.

`, app)

	case "reindex":
		return fmt.Sprintf(`Usage:
  %s reindex [--path <dir>]

`, app)

	case "describe":
		return fmt.Sprintf(`Usage:
  %s describe [--path <dir>] <id>

`, app)

	case "show":
		return fmt.Sprintf(`Usage:
  %s show [--path <dir>] [--all] <id>

Flags:
  --all   show full metadata

`, app)

	case "update":
		return fmt.Sprintf(`Usage:
  %s update [--path <dir>] <id> [<id> ...] [flags]

Flags:
  --title <t>           set new title
  --due <date>          set due date (format depends on date_locale config)
  --project <name>      set project name
  --add-tag <tag>       repeatable
  --remove-tag <tag>    repeatable

`, app)

	case "path":
		return fmt.Sprintf(`Usage:
  %s path [--path <dir>] <thread-id>

Prints the canonical filesystem path for the thread directory.
Accepts either a durable thread ID or a short ID.

Flags:
  --path <dir>   custom workspace path

`, app)

	case "attach":
		return fmt.Sprintf(`Usage:
  %s attach [--path <dir>] <thread-id>

Attach an inline note to a thread. Opens your editor to capture note content.

The note is stored as a content-addressed blob and recorded in attachments.jsonl.

Flags:
  --path <dir>   custom workspace path

Environment variables:
  TK_EDITOR      editor to use (defaults to $EDITOR, then vi)
  EDITOR         editor to use (if TK_EDITOR not set)

`, app)

	default:
		return fmt.Sprintf("Unknown command %q\n\n%s", cmd, usage(app))
	}
}

// validateAliases filters and validates aliases:
// - Removes aliases that conflict with built-in commands (built-in wins)
// - Removes aliases that point to non-existent commands
// - Removes aliases that point to other aliases (no recursion)
// Returns a validated map of alias -> built-in command.
func validateAliases(raw config.Aliases, builtInCommands map[string]bool, verbose bool, errOut io.Writer) config.Aliases {
	valid := make(config.Aliases)

	for alias, target := range raw {
		// Skip aliases that conflict with built-in commands
		if builtInCommands[alias] {
			if verbose {
				fmt.Fprintf(errOut, "Warning: alias %q conflicts with built-in command, ignoring\n", alias)
			}
			continue
		}

		// Check if target is a built-in command
		if !builtInCommands[target] {
			// Check if target is another alias (recursion)
			if _, isAlias := raw[target]; isAlias {
				if verbose {
					fmt.Fprintf(errOut, "Warning: alias %q points to another alias %q (recursion not allowed), ignoring\n", alias, target)
				}
				continue
			}
			// Target is not a built-in and not an alias - invalid
			if verbose {
				fmt.Fprintf(errOut, "Warning: alias %q points to non-existent command %q, ignoring\n", alias, target)
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
