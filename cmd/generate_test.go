package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestGeneratedCompletionsCoverSupportedShells(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "nu"} {
		generated, err := generatedCompletion(shell)
		if err != nil {
			t.Fatalf("generate %s: %v", shell, err)
		}
		for _, want := range []string{"active", "archived"} {
			if !strings.Contains(generated, want) {
				t.Fatalf("%s completion omits %q", shell, want)
			}
		}
	}
}

func TestNushellCompletionIncludesOneRootWrapper(t *testing.T) {
	generated, err := generatedCompletion("nu")
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(generated, "export def --env sy ["); count != 1 {
		t.Fatalf("Nushell integration defines %d root wrappers", count)
	}
	if strings.Contains(generated, `export extern "sy" [`) {
		t.Fatal("Nushell integration retains a duplicate root extern")
	}
	for _, expected := range []string{`string@"__sy_completion_values_0"`, "$args.0 not-in", "return (^sy --greedy $greedy"} {
		if !strings.Contains(generated, expected) {
			t.Fatalf("Nushell integration omits %q", expected)
		}
	}
}

func TestShellIntegrationsReserveCommandsAndAliases(t *testing.T) {
	templates := map[string]string{"bash": bashIntegration, "zsh": zshIntegration, "fish": fishIntegration}
	for shell, source := range templates {
		generated, err := renderShellIntegration(shell, source)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range rootInvocationNames() {
			if !strings.Contains(generated, name) {
				t.Errorf("%s integration does not reserve %q", shell, name)
			}
		}
	}
}

func TestCompletionValuesUseActiveArchiveAndRepositorySources(t *testing.T) {
	state := t.TempDir()
	configHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	for _, path := range []string{
		filepath.Join(state, "seshy", "sessions", "active-work"),
		filepath.Join(state, "seshy", "archive", "finished-work"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(configHome, "seshy"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "repoSource: printf '/src/api\\n/src/web\\n'\n"
	if err := os.WriteFile(filepath.Join(configHome, "seshy", "config.yaml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	active, err := completionValues("active", "")
	if err != nil || !slices.Equal(active, []string{"active-work"}) {
		t.Fatalf("active values = %v, %v", active, err)
	}
	archived, err := completionValues("archived", "")
	if err != nil || !slices.Equal(archived, []string{"finished-work"}) {
		t.Fatalf("archived values = %v, %v", archived, err)
	}
	repositories, err := completionValues("repositories", "")
	if err != nil || !slices.Equal(repositories, []string{"/src/api", "/src/web"}) {
		t.Fatalf("repository values = %v, %v", repositories, err)
	}
	deleteArchived, err := completionValues("sessions", "sy delete --archived ")
	if err != nil || !slices.Equal(deleteArchived, archived) {
		t.Fatalf("archived delete values = %v, %v", deleteArchived, err)
	}
}

func TestContextualCompletionSelectsArguments(t *testing.T) {
	tests := []struct {
		context string
		command string
		want    []string
	}{
		{"sy add ", "add", nil},
		{"sy add act", "add", nil},
		{"sy add active-work ", "add", []string{"active-work"}},
		{"sy add active-work /src/a", "add", []string{"active-work"}},
		{"sy add --branch topic active-work ", "add", []string{"active-work"}},
		{"sy new auth-hardening ", "new", []string{"auth-hardening"}},
		{"sy remove active-work ", "remove", []string{"active-work"}},
	}
	for _, test := range tests {
		got := commandArguments(test.context, test.command, map[string]bool{"--branch": true, "-b": true})
		if !slices.Equal(got, test.want) {
			t.Errorf("commandArguments(%q) = %v, want %v", test.context, got, test.want)
		}
	}
}

func TestConfigInitCreatesCustomConfigParent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "custom", "config.yaml")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))
	t.Setenv("SESHY_CONFIG", path)
	if err := configInitCmd.RunE(configInitCmd, nil); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "archiveDir:") {
		t.Fatalf("generated config omits archiveDir: %s", content)
	}
}

func TestConfigRecoveryCommandsSkipStrictValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("unknownField: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SESHY_CONFIG", path)
	for _, command := range []*cobra.Command{configEditCmd, configInitCmd} {
		if err := rootCmd.PersistentPreRunE(command, nil); err != nil {
			t.Fatalf("%s must remain available to repair invalid config: %v", command.CommandPath(), err)
		}
	}
	if err := rootCmd.PersistentPreRunE(listCmd, nil); err == nil {
		t.Fatal("normal command accepted unknown config field")
	}
}
