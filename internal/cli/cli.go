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
			// Check if tasks directory exists
			if _, err := os.Stat(paths.TasksDir); err == nil {
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
	case "remove", "rm":
		return commands.RunRemove(args, commands.CommandContext{
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
		return runUpdate(args, cfg)

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
  done      Mark one or more tasks done
  remove    Remove one or more tasks (hard delete; requires --force)
  rm        Alias for remove
  reindex   Reassign short IDs for active tasks
  describe  Edit a task description in $EDITOR (later)
  show      Show details for a single task
  update    Update fields on one or more tasks
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
  --due <YYYY-MM-DD>     due date (string for now)
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

	case "remove", "rm":
		return fmt.Sprintf(`Usage:
  %s remove [--path <dir>] --force <id> [<id> ...]
  %s rm     [--path <dir>] --force <id> [<id> ...]

Flags:
  --force   actually delete (required)

`, app, app)

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
  --due <YYYY-MM-DD>    set due date
  --project <name>      set project name
  --add-tag <tag>       repeatable
  --remove-tag <tag>    repeatable

`, app)

	default:
		return fmt.Sprintf("Unknown command %q\n\n%s", cmd, usage(app))
	}
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// --------------------- Commands (acknowledgement only) ---------------------

func runUpdate(argv []string, cfg Config) int {
	fs := flag.NewFlagSet(cfg.AppName+" update", flag.ContinueOnError)
	fs.SetOutput(cfg.Err)
	fs.Usage = func() { fmt.Fprintln(cfg.Err, commandUsage(cfg.AppName, "update")) }

	var (
		path       string
		title      string
		due        string
		project    string
		addTags    stringList
		removeTags stringList
	)

	fs.StringVar(&path, "path", "", "custom workspace path")
	fs.StringVar(&title, "title", "", "set new title")
	fs.StringVar(&due, "due", "", "set due date (YYYY-MM-DD)")
	fs.StringVar(&project, "project", "", "set project name")
	fs.Var(&addTags, "add-tag", "repeatable tag to add")
	fs.Var(&removeTags, "remove-tag", "repeatable tag to remove")

	if err := fs.Parse(argv); err != nil {
		fmt.Fprintln(cfg.Err)
		fmt.Fprintln(cfg.Err, commandUsage(cfg.AppName, "update"))
		return 2
	}

	ids := fs.Args()
	if len(ids) == 0 {
		fmt.Fprintln(cfg.Err, commandUsage(cfg.AppName, "update"))
		return 2
	}

	fmt.Fprintf(cfg.Out, "Would update tasks: path=%q ids=%v title=%q due=%q project=%q addTags=%v removeTags=%v\n",
		path, ids, title, due, project, []string(addTags), []string(removeTags))
	return 0
}
