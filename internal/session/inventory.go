package session

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/roshbhatia/go-utils/git"
)

type WorktreeInventory struct {
	Version      string                `json:"version" jsonschema:"enum=seshy.worktrees/v1"`
	Repositories []RepositoryInventory `json:"repositories"`
	Errors       []string              `json:"errors"`
}

type RepositoryInventory struct {
	CommonDir      string                   `json:"commonDir"`
	SharedGitBytes *int64                   `json:"sharedGitBytes" jsonschema:"nullable"`
	Worktrees      []WorktreeInventoryEntry `json:"worktrees"`
}

type WorktreeInventoryEntry struct {
	Path          string   `json:"path"`
	Head          string   `json:"head"`
	Branch        string   `json:"branch"`
	Sessions      []string `json:"sessions"`
	Bare          bool     `json:"bare"`
	Detached      bool     `json:"detached"`
	Locked        bool     `json:"locked"`
	Prunable      bool     `json:"prunable"`
	Missing       bool     `json:"missing"`
	Dirty         *bool    `json:"dirty" jsonschema:"nullable"`
	Upstream      string   `json:"upstream"`
	Ahead         *int     `json:"ahead" jsonschema:"nullable"`
	Behind        *int     `json:"behind" jsonschema:"nullable"`
	CheckoutBytes *int64   `json:"checkoutBytes" jsonschema:"nullable"`
	Cautions      []string `json:"cautions"`
}

func inventorySeeds(explicit []string) ([]string, map[string][]string, error) {
	active, err := List()
	if err != nil {
		return nil, nil, err
	}
	archived, err := ListArchived()
	if err != nil {
		return nil, nil, err
	}
	membership := map[string][]string{}
	seeds := append([]string{}, explicit...)
	for _, s := range append(active, archived...) {
		entries, err := os.ReadDir(s.Path)
		if err != nil {
			return nil, nil, err
		}
		for _, entry := range entries {
			path := filepath.Join(s.Path, entry.Name())
			if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
					continue
				}
				return nil, nil, err
			}
			membership[realPath(path)] = append(membership[realPath(path)], s.Name)
			if len(explicit) == 0 {
				seeds = append(seeds, path)
			}
		}
	}
	if len(explicit) == 0 {
		if cwd, err := os.Getwd(); err == nil && git.IsRepo(cwd) {
			seeds = append(seeds, cwd)
		}
	}
	for path := range membership {
		sort.Strings(membership[path])
	}
	return seeds, membership, nil
}

func Inventory(repoArgs []string, diskUsage bool) (WorktreeInventory, error) {
	out := WorktreeInventory{Version: "seshy.worktrees/v1", Repositories: []RepositoryInventory{}, Errors: []string{}}
	seeds, membership, err := inventorySeeds(repoArgs)
	if err != nil {
		out.Errors = append(out.Errors, err.Error())
		return out, err
	}
	seen := map[string]bool{}
	var problems []error
	fail := func(err error) { problems = append(problems, err); out.Errors = append(out.Errors, err.Error()) }
	for _, seed := range seeds {
		common, err := git.CommonDir(seed)
		if err != nil {
			fail(err)
			continue
		}
		common = realPath(common)
		if seen[common] {
			continue
		}
		seen[common] = true
		trees, err := git.Worktrees(seed)
		if err != nil {
			fail(err)
			continue
		}
		repo := RepositoryInventory{CommonDir: common, Worktrees: []WorktreeInventoryEntry{}}
		excluded := map[string]bool{common: true}
		for _, tree := range trees {
			excluded[realPath(tree.Path)] = true
		}
		if diskUsage {
			size, err := allocatedBytes(common, nil)
			if err != nil {
				fail(err)
			} else {
				repo.SharedGitBytes = &size
			}
		}
		for _, tree := range trees {
			row := WorktreeInventoryEntry{Path: tree.Path, Head: tree.HEAD, Branch: strings.TrimPrefix(tree.Branch, "refs/heads/"), Sessions: []string{}, Bare: tree.Bare, Detached: tree.Detached, Locked: tree.Locked, Prunable: tree.Prunable, Cautions: []string{}}
			row.Sessions = append(row.Sessions, membership[realPath(tree.Path)]...)
			if len(row.Sessions) == 0 {
				row.Cautions = append(row.Cautions, "external: lifecycle owner not established")
			}
			if row.Locked {
				row.Cautions = append(row.Cautions, "locked: "+tree.LockReason)
			}
			if row.Prunable {
				row.Cautions = append(row.Cautions, "prunable: "+tree.PruneReason)
			}
			_, statErr := os.Stat(tree.Path)
			row.Missing = errors.Is(statErr, os.ErrNotExist)
			if row.Missing {
				row.Cautions = append(row.Cautions, "missing directory")
			} else if statErr != nil {
				fail(statErr)
			} else if !tree.Bare {
				status, err := git.Output(tree.Path, "--no-optional-locks", "status", "--porcelain", "--untracked-files=normal")
				if err != nil {
					fail(err)
					row.Cautions = append(row.Cautions, "status unavailable")
				} else {
					dirty := status != ""
					row.Dirty = &dirty
					if dirty {
						row.Cautions = append(row.Cautions, "uncommitted or untracked files")
					}
				}
				if tree.Detached {
					row.Cautions = append(row.Cautions, "detached HEAD: preserve commits before removal")
				} else {
					upstream, err := git.Output(tree.Path, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
					if err != nil {
						row.Cautions = append(row.Cautions, "no resolved upstream")
					} else {
						row.Upstream = upstream
						counts, err := git.Output(tree.Path, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
						if err != nil {
							fail(err)
						} else {
							fields := strings.Fields(counts)
							if len(fields) != 2 {
								fail(fmt.Errorf("invalid ahead/behind counts for %s", tree.Path))
							} else {
								ahead, e1 := strconv.Atoi(fields[0])
								behind, e2 := strconv.Atoi(fields[1])
								if e1 != nil || e2 != nil {
									fail(fmt.Errorf("invalid ahead/behind counts for %s", tree.Path))
								} else {
									row.Ahead = &ahead
									row.Behind = &behind
									if ahead > 0 {
										row.Cautions = append(row.Cautions, "commits ahead of local upstream ref")
									}
								}
							}
						}
					}
				}
				if diskUsage {
					size, err := allocatedBytes(realPath(tree.Path), excluded)
					if err != nil {
						fail(err)
					} else {
						row.CheckoutBytes = &size
					}
				}
			}
			repo.Worktrees = append(repo.Worktrees, row)
		}
		out.Repositories = append(out.Repositories, repo)
	}
	sort.Slice(out.Repositories, func(i, j int) bool { return out.Repositories[i].CommonDir < out.Repositories[j].CommonDir })
	return out, errors.Join(problems...)
}

func allocatedBytes(root string, excluded map[string]bool) (int64, error) {
	var size int64
	seen := map[[2]uint64]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != root && excluded[path] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf("allocated size unavailable for %s", path)
		}
		key := [2]uint64{uint64(stat.Dev), uint64(stat.Ino)}
		if !seen[key] {
			size += stat.Blocks * 512
			seen[key] = true
		}
		return nil
	})
	return size, err
}

func PruneMetadata(repoArgs []string, dryRun bool) ([]PruneAction, error) {
	seeds, _, err := inventorySeeds(repoArgs)
	if err != nil {
		return nil, err
	}
	var actions []PruneAction
	var problems []error
	seen := map[string]bool{}
	for _, repo := range seeds {
		common, err := git.CommonDir(repo)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		common = realPath(common)
		if seen[common] {
			continue
		}
		seen[common] = true
		candidates, err := prunableWorktrees(repo)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		if dryRun {
			actions = append(actions, candidates...)
			continue
		}
		if len(candidates) == 0 {
			continue
		}
		if err := git.WorktreePrune(repo); err != nil {
			problems = append(problems, err)
			continue
		}
		remaining, err := git.Worktrees(repo)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		registered := map[string]bool{}
		for _, tree := range remaining {
			registered[realPath(tree.Path)] = true
		}
		for _, candidate := range candidates {
			if !registered[realPath(candidate.Target)] {
				actions = append(actions, candidate)
			}
		}
	}
	return actions, errors.Join(problems...)
}
