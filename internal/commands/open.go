package commands

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/sjatkinson/threadkeeper/internal/config"
	"github.com/sjatkinson/threadkeeper/internal/store"
)

// FileOpener abstracts platform-specific file opening.
// This interface allows testing without actually executing OS commands.
type FileOpener interface {
	OpenFile(path string) error
	OpenURL(url string) error
}

// detectPlatform returns the current platform identifier.
func detectPlatform() string {
	return runtime.GOOS
}

// newFileOpener creates a platform-specific file opener.
// Returns an error if the platform is not supported.
func newFileOpener() (FileOpener, error) {
	platform := detectPlatform()
	switch platform {
	case "darwin":
		return &macOpener{}, nil
	case "linux":
		return &linuxOpener{}, nil
	case "windows":
		return &windowsOpener{}, nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
}

// macOpener implements FileOpener for macOS using the "open" command.
type macOpener struct{}

func (o *macOpener) OpenFile(path string) error {
	return exec.Command("open", path).Run()
}

func (o *macOpener) OpenURL(url string) error {
	return exec.Command("open", url).Run()
}

// linuxOpener implements FileOpener for Linux using "xdg-open".
type linuxOpener struct{}

func (o *linuxOpener) OpenFile(path string) error {
	return exec.Command("xdg-open", path).Run()
}

func (o *linuxOpener) OpenURL(url string) error {
	return exec.Command("xdg-open", url).Run()
}

// windowsOpener implements FileOpener for Windows using "cmd /c start".
type windowsOpener struct{}

func (o *windowsOpener) OpenFile(path string) error {
	return exec.Command("cmd", "/c", "start", "", path).Run()
}

func (o *windowsOpener) OpenURL(url string) error {
	return exec.Command("cmd", "/c", "start", "", url).Run()
}

func RunOpen(args []string, ctx CommandContext) int {
	fs := flag.NewFlagSet(ctx.AppName+" open", flag.ContinueOnError)
	fs.SetOutput(ctx.Err)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(ctx.Err, openUsage(ctx.AppName))
	}

	var (
		path      string
		attIndex  int
		attID     string
		printPath bool
	)

	fs.StringVar(&path, "path", "", "custom workspace path")
	fs.IntVar(&attIndex, "att", 0, "attachment index (1-based)")
	fs.StringVar(&attID, "att-id", "", "attachment ID (alternative to --att)")
	fs.BoolVar(&printPath, "print-path", false, "print path instead of opening")

	// Preprocess args: if first arg looks like a thread ID (not a flag), move it to the end
	// This allows both "tk open <id> --att 1" and "tk open --att 1 <id>"
	processedArgs := make([]string, 0, len(args))
	var threadIDCandidate string
	for i, arg := range args {
		if i == 0 && !strings.HasPrefix(arg, "-") && threadIDCandidate == "" {
			// First non-flag arg might be thread ID - save it for later
			threadIDCandidate = arg
		} else {
			processedArgs = append(processedArgs, arg)
		}
	}
	// If we found a thread ID candidate at the start, append it at the end
	if threadIDCandidate != "" {
		processedArgs = append(processedArgs, threadIDCandidate)
	}

	if err := fs.Parse(processedArgs); err != nil {
		_, _ = fmt.Fprintln(ctx.Err)
		_, _ = fmt.Fprintln(ctx.Err, openUsage(ctx.AppName))
		return 2
	}

	rest := fs.Args()
	if len(rest) != 1 {
		_, _ = fmt.Fprintf(ctx.Err, "Error: missing argument: thread ID required\n")
		return 2
	}

	threadIDStr := rest[0]

	// Validate that either --att or --att-id is provided
	if attIndex == 0 && attID == "" {
		_, _ = fmt.Fprintf(ctx.Err, "Error: must specify either --att <index> or --att-id <id>\n")
		return 2
	}

	if attIndex != 0 && attID != "" {
		_, _ = fmt.Fprintf(ctx.Err, "Error: cannot specify both --att and --att-id\n")
		return 2
	}

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

	// Resolve thread ID
	st := store.NewFileStore(paths.ThreadsDir)
	t, err := st.ResolveID(threadIDStr)
	if err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	// Get thread directory path
	threadDir := store.ThreadPath(paths.ThreadsDir, t.ID)

	// Load attachments
	attachments, err := loadAttachments(threadDir)
	if err != nil {
		if !os.IsNotExist(err) {
			_, _ = fmt.Fprintf(ctx.Err, "Warning: failed to load attachments: %v\n", err)
		}
		attachments = []AttachmentEvent{}
	}

	// Compute current attachments (for indexing)
	currentAtts := computeCurrentAttachments(attachments)

	// Find target attachment
	var target *AttachmentEvent
	if attID != "" {
		// Find by ID
		for i := range currentAtts {
			if currentAtts[i].Att.AttID == attID {
				target = &currentAtts[i]
				break
			}
		}
		if target == nil {
			_, _ = fmt.Fprintf(ctx.Err, "Error: attachment with ID %q not found\n", attID)
			return 1
		}
	} else {
		// Find by index (1-based)
		if attIndex < 1 {
			_, _ = fmt.Fprintf(ctx.Err, "Error: attachment index must be >= 1\n")
			return 2
		}
		if attIndex > len(currentAtts) {
			_, _ = fmt.Fprintf(ctx.Err, "Error: attachment index %d out of range (max: %d)\n", attIndex, len(currentAtts))
			return 1
		}
		target = &currentAtts[attIndex-1]
	}

	// Handle link attachments (open URL)
	if target.Att.Kind == "link" {
		if target.Att.URL == "" {
			_, _ = fmt.Fprintf(ctx.Err, "Error: link attachment has no URL\n")
			return 1
		}

		// Print URL or open it
		if printPath {
			_, _ = fmt.Fprintln(ctx.Out, target.Att.URL)
			return 0
		}

		// Open URL using platform-specific opener
		opener, err := newFileOpener()
		if err != nil {
			_, _ = fmt.Fprintf(ctx.Err, "Error: %v\n", err)
			return 1
		}

		if err := opener.OpenURL(target.Att.URL); err != nil {
			_, _ = fmt.Fprintf(ctx.Err, "Error: failed to open URL: %v\n", err)
			return 1
		}

		return 0
	}

	// Handle note attachments (open blob file)
	if target.Att.Blob == nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: note attachment has no blob reference\n")
		return 1
	}

	blobPath := blobPath(threadDir, *target.Att.Blob)
	if blobPath == "" {
		_, _ = fmt.Fprintf(ctx.Err, "Error: unsupported blob algorithm %q\n", target.Att.Blob.Algo)
		return 1
	}

	// Check if blob file exists
	if _, err := os.Stat(blobPath); err != nil {
		if os.IsNotExist(err) {
			_, _ = fmt.Fprintf(ctx.Err, "Error: blob file not found at %s\n", blobPath)
			return 1
		}
		_, _ = fmt.Fprintf(ctx.Err, "Error: failed to access blob file: %v\n", err)
		return 1
	}

	// Print path or open file
	if printPath {
		_, _ = fmt.Fprintln(ctx.Out, blobPath)
		return 0
	}

	// Open file using platform-specific opener
	opener, err := newFileOpener()
	if err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: %v\n", err)
		return 1
	}

	if err := opener.OpenFile(blobPath); err != nil {
		_, _ = fmt.Fprintf(ctx.Err, "Error: failed to open file: %v\n", err)
		return 1
	}

	return 0
}

func openUsage(app string) string {
	return fmt.Sprintf(`Usage:
  %s open [--path <dir>] [--att <index> | --att-id <id>] [--print-path] <thread-id>

Open an attachment from a thread.

Flags:
  --path <dir>      custom workspace path
  --att <index>     attachment index (1-based, from 'show' output)
  --att-id <id>     attachment ID (alternative to --att)
  --print-path      print blob path instead of opening

Examples:
  %s open 1 --att 1
  %s open 1 --att-id 01ARZ3NDEKTSV4RRFFQ69G5FAV --print-path

`, app, app, app)
}
