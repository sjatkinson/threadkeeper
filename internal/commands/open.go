package commands

import (
	"fmt"
	"os/exec"
	"runtime"
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
