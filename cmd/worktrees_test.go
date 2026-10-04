package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreesOutputMatchesSchema(t *testing.T) {
	isolatedRoot(t)
	repo := filepath.Join(t.TempDir(), "repo")
	setupGitRepo(t, repo)
	stdout, _, err := runCmd("worktrees", repo, "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	validate(t, compileSchema(t, worktreesSchemaFile), stdout)
	if !strings.Contains(stdout, "seshy.worktrees/v1") {
		t.Fatal("missing document")
	}
}

func TestSparseAndReferenceAreExclusive(t *testing.T) {
	isolatedRoot(t)
	_, _, err := runCmd("new", "invalid", "--reference", "--sparse-directory", "src", "--empty")
	if err == nil {
		t.Fatal("accepted incompatible flags")
	}
}
