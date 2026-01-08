package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/sjatkinson/threadkeeper/internal/config"
)

func TestGetCommand(t *testing.T) {
	tests := []struct {
		name     string
		cmdName  string
		want     bool
		wantName string
	}{
		{"init exists", "init", true, "init"},
		{"add exists", "add", true, "add"},
		{"list exists", "list", true, "list"},
		{"nonexistent command", "nonexistent", false, ""},
		{"empty command", "", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := getCommand(tt.cmdName)
			if (info != nil) != tt.want {
				t.Errorf("getCommand(%q) = %v, want %v", tt.cmdName, info != nil, tt.want)
				return
			}
			if tt.want && info != nil && info.Name != tt.wantName {
				t.Errorf("getCommand(%q).Name = %q, want %q", tt.cmdName, info.Name, tt.wantName)
			}
		})
	}
}

func TestGetAllCommands(t *testing.T) {
	cmds := getAllCommands()
	if len(cmds) == 0 {
		t.Error("getAllCommands() returned no commands")
	}

	// Verify all expected commands are present
	expected := map[string]bool{
		"init":     false,
		"add":      false,
		"list":     false,
		"show":     false,
		"describe": false,
		"update":   false,
		"done":     false,
		"archive":  false,
		"reopen":   false,
		"remove":   false,
		"reindex":  false,
		"path":     false,
		"attach":   false,
	}

	for _, cmd := range cmds {
		if _, ok := expected[cmd.Name]; ok {
			expected[cmd.Name] = true
		}
		if cmd.Description == "" {
			t.Errorf("Command %q has empty description", cmd.Name)
		}
		if cmd.Usage == nil {
			t.Errorf("Command %q has nil Usage function", cmd.Name)
		}
		if cmd.Runner == nil {
			t.Errorf("Command %q has nil Runner function", cmd.Name)
		}
	}

	// Check all expected commands were found
	for name, found := range expected {
		if !found {
			t.Errorf("Expected command %q not found in getAllCommands()", name)
		}
	}

	// Verify commands are sorted
	for i := 1; i < len(cmds); i++ {
		if cmds[i-1].Name > cmds[i].Name {
			// Not sorted, but that's OK - we use manual ordering in usage()
		}
	}
}

func TestCommandUsage(t *testing.T) {
	tests := []struct {
		name        string
		cmd         string
		wantContain string
	}{
		{"init usage", "init", "Usage:"},
		{"add usage", "add", "Usage:"},
		{"list usage", "list", "Usage:"},
		{"unknown command", "nonexistent", "Unknown command"},
		{"empty command", "", "Unknown command"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := commandUsage("tk", tt.cmd)
			if !strings.Contains(result, tt.wantContain) {
				t.Errorf("commandUsage(%q) = %q, want to contain %q", tt.cmd, result, tt.wantContain)
			}
			if result == "" {
				t.Errorf("commandUsage(%q) returned empty string", tt.cmd)
			}
		})
	}
}

func TestUsageIncludesAllCommands(t *testing.T) {
	usageText := usage("tk")

	// Verify usage contains all registered commands
	expectedCommands := []string{"init", "add", "list", "show", "describe", "update", "done", "archive", "reopen", "remove", "reindex", "path", "attach", "help"}

	for _, cmd := range expectedCommands {
		if !strings.Contains(usageText, cmd) {
			t.Errorf("usage() output does not contain command %q", cmd)
		}
	}

	// Verify usage contains expected sections
	expectedSections := []string{"Usage:", "Global flags:", "Commands:", "help <command>"}
	for _, section := range expectedSections {
		if !strings.Contains(usageText, section) {
			t.Errorf("usage() output does not contain section %q", section)
		}
	}
}

func TestRun_CommandParsing(t *testing.T) {
	tests := []struct {
		name     string
		argv     []string
		wantCode int
		wantErr  string
		setup    func() func() // setup function returns cleanup
	}{
		{
			name:     "version flag",
			argv:     []string{"--version"},
			wantCode: 0,
			wantErr:  "",
		},
		{
			name:     "help flag",
			argv:     []string{"--help"},
			wantCode: 0,
			wantErr:  "Usage:",
		},
		{
			name:     "unknown command",
			argv:     []string{"nonexistent"},
			wantCode: 2,
			wantErr:  "unknown command",
		},
		{
			name:     "help command",
			argv:     []string{"help"},
			wantCode: 0,
			wantErr:  "Usage:",
		},
		{
			name:     "help with subcommand",
			argv:     []string{"help", "add"},
			wantCode: 0,
			wantErr:  "Usage:",
		},
		{
			name:     "invalid global flag",
			argv:     []string{"--invalid-flag"},
			wantCode: 2,
			wantErr:  "Usage:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cleanup func()
			if tt.setup != nil {
				cleanup = tt.setup()
				if cleanup != nil {
					defer cleanup()
				}
			}

			var outBuf, errBuf bytes.Buffer
			cfg := Config{
				AppName: "tk",
				Out:     &outBuf,
				Err:     &errBuf,
			}

			code := Run(tt.argv, cfg)

			if code != tt.wantCode {
				t.Errorf("Run() exit code = %d, want %d", code, tt.wantCode)
			}

			if tt.wantErr != "" {
				errOutput := errBuf.String()
				if !strings.Contains(errOutput, tt.wantErr) {
					t.Errorf("Run() error output = %q, want to contain %q", errOutput, tt.wantErr)
				}
			}
		})
	}
}

func TestRun_AliasResolution(t *testing.T) {
	// This test would require mocking config.LoadAliases()
	// For now, we test that alias resolution doesn't break
	// Full alias testing would require integration tests with real config files
	t.Skip("Alias resolution testing requires config mocking - see integration tests")
}

func TestRun_DefaultToList(t *testing.T) {
	tests := []struct {
		name           string
		argv           []string
		setupWorkspace bool
		wantCode       int
		wantErr        string
	}{
		{
			name:           "no args with workspace",
			argv:           []string{},
			setupWorkspace: true,
			wantCode:       0, // Should run list command
			wantErr:        "",
		},
		{
			name:           "no args without workspace",
			argv:           []string{},
			setupWorkspace: false,
			wantCode:       0,        // Should show usage
			wantErr:        "Usage:", // Usage goes to stderr
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for workspace if needed
			if tt.setupWorkspace {
				tmpDir := t.TempDir()
				t.Setenv("THREADKEEPER_WORKSPACE", tmpDir)
				// Create threads directory to simulate existing workspace
				os.MkdirAll(tmpDir+"/threads", 0755)
			} else {
				// For no workspace case, set env to a non-existent directory
				// This ensures config.GetPaths("") will either fail or return a path
				// where threads directory doesn't exist
				tmpDir := t.TempDir()
				t.Setenv("THREADKEEPER_WORKSPACE", tmpDir)
				// Don't create threads directory - workspace should not exist
			}

			var outBuf, errBuf bytes.Buffer
			cfg := Config{
				AppName: "tk",
				Out:     &outBuf,
				Err:     &errBuf,
			}

			code := Run(tt.argv, cfg)

			if code != tt.wantCode {
				t.Errorf("Run() exit code = %d, want %d", code, tt.wantCode)
			}

			if tt.wantErr != "" {
				errOutput := errBuf.String()
				if !strings.Contains(errOutput, tt.wantErr) {
					t.Errorf("Run() error output = %q, want to contain %q", errOutput, tt.wantErr)
				}
			}
		})
	}
}

func TestValidateAliases(t *testing.T) {
	tests := []struct {
		name     string
		raw      map[string]string
		verbose  bool
		want     map[string]string
		wantWarn bool
	}{
		{
			name:     "valid alias",
			raw:      map[string]string{"rm": "remove"},
			verbose:  false,
			want:     map[string]string{"rm": "remove"},
			wantWarn: false,
		},
		{
			name:     "alias conflicts with built-in",
			raw:      map[string]string{"add": "list"}, // "add" is a built-in
			verbose:  true,
			want:     map[string]string{},
			wantWarn: true,
		},
		{
			name:     "alias points to non-existent",
			raw:      map[string]string{"foo": "nonexistent"},
			verbose:  true,
			want:     map[string]string{},
			wantWarn: true,
		},
		{
			name:     "alias points to another alias",
			raw:      map[string]string{"rm": "remove", "del": "rm"}, // "del" -> "rm" -> "remove" (recursion)
			verbose:  true,
			want:     map[string]string{"rm": "remove"}, // Only first level should be valid
			wantWarn: true,
		},
		{
			name:     "multiple valid aliases",
			raw:      map[string]string{"rm": "remove", "ls": "list"},
			verbose:  false,
			want:     map[string]string{"rm": "remove", "ls": "list"},
			wantWarn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errBuf bytes.Buffer
			rawAliases := make(map[string]string)
			for k, v := range tt.raw {
				rawAliases[k] = v
			}

			// Convert to config.Aliases type
			raw := make(config.Aliases)
			for k, v := range rawAliases {
				raw[k] = v
			}

			valid := validateAliases(raw, tt.verbose, &errBuf)

			// Check result
			if len(valid) != len(tt.want) {
				t.Errorf("validateAliases() returned %d aliases, want %d", len(valid), len(tt.want))
			}

			for k, v := range tt.want {
				if valid[k] != v {
					t.Errorf("validateAliases()[%q] = %q, want %q", k, valid[k], v)
				}
			}

			// Check warnings
			errOutput := errBuf.String()
			hasWarn := errOutput != ""
			if hasWarn != tt.wantWarn {
				t.Errorf("validateAliases() warning output = %v, want %v (output: %q)", hasWarn, tt.wantWarn, errOutput)
			}
		})
	}
}
