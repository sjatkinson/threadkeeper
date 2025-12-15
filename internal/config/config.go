package config

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	AppDirName = "threadkeeper"

	// Env var for overriding workspace dir (CLI still wins).
	WorkspaceEnvVar = "THREADKEEPER_WORKSPACE"

	// Key we read from config.toml
	DefaultWorkspaceKey = "default_workspace"
)

type Paths struct {
	Workspace  string
	ThreadsDir string
	// Later: AttachmentsDir, NotesDir, IndexDir, etc.
}

// ConfigPath returns the config file path:
//
//	$XDG_CONFIG_HOME/threadkeeper/config.toml
//
// or
//
//	~/.config/threadkeeper/config.toml
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(home, ".config")
	}

	return filepath.Join(base, AppDirName, "config.toml"), nil
}

// DefaultDataDir returns the XDG-ish default data directory:
//
//	$XDG_DATA_HOME/threadkeeper
//
// or
//
//	~/.local/share/threadkeeper
func DefaultDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		base = filepath.Join(home, ".local", "share")
	}

	return filepath.Join(base, AppDirName), nil
}

// LoadDefaultWorkspace reads config.toml and returns the value of
// default_workspace if present. This is a minimal parser:
//
//	default_workspace = "/some/path"
//	default_workspace = '~/path'
//
// It ignores comments and other keys.
func LoadDefaultWorkspace() (string, bool, error) {
	cfgPath, err := ConfigPath()
	if err != nil {
		return "", false, err
	}

	f, err := os.Open(cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip inline comments: key = "x" # comment
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if !strings.Contains(line, "=") {
			continue
		}

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key != DefaultWorkspaceKey {
			continue
		}

		val = strings.TrimSpace(val)
		// Remove optional surrounding quotes
		val = strings.Trim(val, `"'`)
		val = strings.TrimSpace(val)
		if val == "" {
			return "", false, nil
		}

		val, err = ExpandUser(val)
		if err != nil {
			return "", false, err
		}
		return val, true, nil
	}

	if err := sc.Err(); err != nil {
		return "", false, err
	}
	return "", false, nil
}

// WorkspacePath returns the workspace directory based on precedence:
// custom CLI path > env var > config > XDG default
func WorkspacePath(custom string) (string, error) {
	// 1) CLI
	if strings.TrimSpace(custom) != "" {
		return ExpandUser(custom)
	}

	// 2) Env var
	if env := strings.TrimSpace(os.Getenv(WorkspaceEnvVar)); env != "" {
		return ExpandUser(env)
	}

	// 3) Config
	if cfg, ok, err := LoadDefaultWorkspace(); err != nil {
		return "", err
	} else if ok {
		return cfg, nil
	}

	// 4/5) XDG default
	return DefaultDataDir()
}

func GetPaths(custom string) (Paths, error) {
	ws, err := WorkspacePath(custom)
	if err != nil {
		return Paths{}, err
	}

	ws = filepath.Clean(ws)
	return Paths{
		Workspace:  ws,
		ThreadsDir: filepath.Join(ws, "threads"),
	}, nil
}

// ExpandUser expands a leading "~/" to the user home directory.
// If the path doesn't start with "~", it returns it unchanged.
func ExpandUser(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", nil
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			return home, nil
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}

// Aliases is a map of alias name to target command.
type Aliases map[string]string

// LoadAliases reads config.toml and returns aliases from the [alias] section.
// Returns an empty map (not an error) if:
//   - Config file doesn't exist
//   - [alias] section doesn't exist
//   - [alias] section is empty
//
// Returns an error only if the config file exists but is malformed TOML.
func LoadAliases() (Aliases, error) {
	cfgPath, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(Aliases), nil
		}
		return nil, err
	}

	var cfg struct {
		Alias map[string]string `toml:"alias"`
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		// Malformed TOML - return error
		return nil, err
	}

	if cfg.Alias == nil {
		return make(Aliases), nil
	}

	// Return a copy to avoid external modification
	aliases := make(Aliases, len(cfg.Alias))
	for k, v := range cfg.Alias {
		aliases[k] = v
	}

	return aliases, nil
}
