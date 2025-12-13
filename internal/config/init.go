package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type InitOptions struct {
	CustomPath string
	Force      bool
}

type InitResult struct {
	Workspace string
	TasksDir  string
	Existed   bool // true if tasks dir already existed before init
}

func InitWorkspace(opts InitOptions) (InitResult, error) {
	paths, err := GetPaths(opts.CustomPath)
	if err != nil {
		return InitResult{}, err
	}

	// Ensure workspace exists
	if err := os.MkdirAll(paths.Workspace, 0o755); err != nil {
		return InitResult{}, err
	}

	// Tasks dir handling
	existed := dirExists(paths.TasksDir)

	// If tasks dir exists and is non-empty, refuse unless --force
	if dirHasRegularFiles(paths.TasksDir) && !opts.Force {
		return InitResult{}, fmt.Errorf(
			"tasks directory %s exists and is not empty (use --force to reinitialize)",
			paths.TasksDir,
		)
	}

	// If force, delete regular files in tasks dir (create dir first if needed)
	if opts.Force {
		if err := os.MkdirAll(paths.TasksDir, 0o755); err != nil {
			return InitResult{}, err
		}
		if err := deleteRegularFiles(paths.TasksDir); err != nil {
			return InitResult{}, err
		}
	}

	// Ensure tasks dir exists
	if err := os.MkdirAll(paths.TasksDir, 0o755); err != nil {
		return InitResult{}, err
	}

	return InitResult{
		Workspace: paths.Workspace,
		TasksDir:  paths.TasksDir,
		Existed:   existed,
	}, nil
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

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
