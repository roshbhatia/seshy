package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roshbhatia/go-utils/git"
)

func inventoryGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if err := git.Run(dir, args...); err != nil {
		t.Fatal(err)
	}
}

func TestInventoryFindsExternalDetachedAndSharedStorage(t *testing.T) {
	isolatedRoot(t)
	repo := filepath.Join(t.TempDir(), "source")
	setupTestGitRepo(t, repo)
	external := filepath.Join(t.TempDir(), "codex tree")
	inventoryGit(t, repo, "worktree", "add", "--detach", external, "HEAD")
	if err := os.WriteFile(filepath.Join(external, "untracked"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Create("references", []string{external}, CreateOpts{Reference: true}); err != nil {
		t.Fatal(err)
	}
	report, err := Inventory([]string{repo, external}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Repositories) != 1 || len(report.Repositories[0].Worktrees) != 2 {
		t.Fatalf("duplicate or missing trees: %+v", report)
	}
	r := report.Repositories[0]
	if r.SharedGitBytes == nil || *r.SharedGitBytes == 0 {
		t.Fatal("missing shared storage")
	}
	tree := r.Worktrees[1]
	if !tree.Detached || tree.Dirty == nil || !*tree.Dirty || len(tree.Sessions) != 1 || tree.Sessions[0] != "references" {
		t.Fatalf("wrong external state: %+v", tree)
	}
	if tree.CheckoutBytes == nil || *tree.CheckoutBytes == 0 {
		t.Fatal("missing checkout storage")
	}
	if _, err := os.Stat(filepath.Join(external, "untracked")); err != nil {
		t.Fatal("inventory changed worktree")
	}
}

func TestInventoryDefaultIncludesArchivedSources(t *testing.T) {
	isolatedRoot(t)
	repo := filepath.Join(t.TempDir(), "source")
	setupTestGitRepo(t, repo)
	if _, err := Create("old", []string{repo}, CreateOpts{Reference: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := Archive("old"); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	out, err := Inventory(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Repositories) != 1 || out.Repositories[0].SharedGitBytes != nil {
		t.Fatalf("wrong inventory: %+v", out)
	}
}

func TestInventoryAheadMissingAndLocked(t *testing.T) {
	isolatedRoot(t)
	repo := filepath.Join(t.TempDir(), "source")
	setupTestGitRepo(t, repo)
	inventoryGit(t, repo, "branch", "base")
	inventoryGit(t, repo, "branch", "--set-upstream-to=base")
	inventoryGit(t, repo, "commit", "--allow-empty", "-m", "ahead")
	path := filepath.Join(t.TempDir(), "offline")
	inventoryGit(t, repo, "worktree", "add", "--detach", path)
	inventoryGit(t, repo, "worktree", "lock", "--reason", "offline", path)
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	out, err := Inventory([]string{repo}, false)
	if err != nil {
		t.Fatal(err)
	}
	trees := out.Repositories[0].Worktrees
	if trees[0].Ahead == nil || *trees[0].Ahead != 1 {
		t.Fatalf("wrong ahead count: %+v", trees[0])
	}
	if !trees[1].Missing || !trees[1].Locked || trees[1].Dirty != nil {
		t.Fatalf("wrong offline state: %+v", trees[1])
	}
}

func TestPruneMetadataPreservesBranchesReferencesAndLockedTrees(t *testing.T) {
	root := isolatedRoot(t)
	repo := filepath.Join(root, "source")
	setupTestGitRepo(t, repo)
	infos, err := Create("gone", []string{repo}, CreateOpts{BranchFormat: "sy/{{.Session}}/{{.Repo}}"})
	if err != nil {
		t.Fatal(err)
	}
	path := infos[0].Path
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(t.TempDir(), "locked")
	inventoryGit(t, repo, "worktree", "add", "--detach", locked)
	inventoryGit(t, repo, "worktree", "lock", locked)
	if err := os.RemoveAll(locked); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(filepath.Dir(path), "dangling")
	if err := os.Symlink(filepath.Join(root, "missing"), link); err != nil {
		t.Fatal(err)
	}
	before, err := git.Worktrees(repo)
	if err != nil {
		t.Fatal(err)
	}
	dry, err := PruneMetadata([]string{repo, repo}, true)
	if err != nil || len(dry) != 1 {
		t.Fatalf("dry run: %+v %v", dry, err)
	}
	after, _ := git.Worktrees(repo)
	if len(before) != len(after) {
		t.Fatal("dry run mutated registrations")
	}
	actions, err := PruneMetadata([]string{repo}, false)
	if err != nil || len(actions) != 1 {
		t.Fatalf("prune: %+v %v", actions, err)
	}
	if !branchExists(t, repo, infos[0].Branch) {
		t.Fatal("deleted branch")
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatal("deleted reference")
	}
	after, _ = git.Worktrees(repo)
	if len(after) != 2 || !after[1].Locked {
		t.Fatalf("lost locked tree: %+v", after)
	}
}

func TestAllocatedBytesDoesNotFollowLinksOrCountNestedWorktree(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "large"), []byte(strings.Repeat("x", 65536)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(nested, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	all, err := allocatedBytes(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	excluded, err := allocatedBytes(root, map[string]bool{nested: true})
	if err != nil {
		t.Fatal(err)
	}
	if all-excluded < 65536 {
		t.Fatalf("nested tree still counted: %d %d", all, excluded)
	}
}

func TestSparseCreationPreservesSourceAndSibling(t *testing.T) {
	isolatedRoot(t)
	repo := filepath.Join(t.TempDir(), "source")
	setupTestGitRepo(t, repo)
	for _, dir := range []string{"included,comma", "excluded"} {
		if err := os.Mkdir(filepath.Join(repo, dir), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, dir, "file"), []byte("tracked"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	inventoryGit(t, repo, "add", ".")
	inventoryGit(t, repo, "commit", "-m", "directories")
	sibling := filepath.Join(t.TempDir(), "sibling")
	inventoryGit(t, repo, "worktree", "add", "--detach", sibling)
	infos, err := Create("sparse", []string{repo}, CreateOpts{BranchFormat: "sy/{{.Session}}/{{.Repo}}", SparseDirectories: []string{"included,comma"}})
	if err != nil {
		t.Fatal(err)
	}
	wt := infos[0].Path
	for _, p := range []string{filepath.Join(wt, "included,comma/file"), filepath.Join(wt, "README.md"), filepath.Join(repo, "excluded/file"), filepath.Join(sibling, "excluded/file")} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
	}
	if _, err := os.Stat(filepath.Join(wt, "excluded")); !os.IsNotExist(err) {
		t.Fatal("full checkout materialized")
	}
	for _, p := range []string{repo, sibling} {
		if sparse, _ := git.Output(p, "config", "--bool", "core.sparseCheckout"); sparse == "true" {
			t.Fatal("changed another worktree sparse config")
		}
	}
}

func TestSparseFailureRollsBackOwnedWorktree(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "new", true: "existing"}[existing], func(t *testing.T) {
			isolatedRoot(t)
			repo := filepath.Join(t.TempDir(), "source")
			setupTestGitRepo(t, repo)
			if existing {
				inventoryGit(t, repo, "branch", "feature")
			}
			// A tracked file is rejected as a cone directory after registration succeeds.
			_, err := Create("bad", []string{repo}, CreateOpts{BranchOverride: "feature", ExistingBranch: existing, SparseDirectories: []string{"README.md"}})
			if err == nil {
				t.Fatal("expected sparse setup failure")
			}
			trees, err := git.Worktrees(repo)
			if err != nil {
				t.Fatal(err)
			}
			if len(trees) != 1 {
				t.Fatalf("leaked registration: %+v", trees)
			}
			if branchExists(t, repo, "feature") != existing {
				t.Fatal("wrong branch rollback")
			}
		})
	}
}

func TestSparseRejectsEscapingPathsBeforeMutation(t *testing.T) {
	isolatedRoot(t)
	repo := filepath.Join(t.TempDir(), "source")
	setupTestGitRepo(t, repo)
	for _, dir := range []string{"../elsewhere", "/absolute", "a/../../b", "a\nb"} {
		if _, err := Create("bad", []string{repo}, CreateOpts{BranchOverride: "feature", SparseDirectories: []string{dir}}); err == nil {
			t.Fatalf("accepted %q", dir)
		}
	}
	if branchExists(t, repo, "feature") {
		t.Fatal("created branch before validation")
	}
}
