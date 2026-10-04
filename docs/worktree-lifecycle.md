# Worktree ownership and storage

Git linked worktrees already share one object database. A separate object cache or alternate is unnecessary for those worktrees.
Checkout files, dependencies, and build output remain separate. Reducing those is usually the useful storage improvement.

Use one session per feature and add repositories as needed. Fetch the intended base before creating a branch:

```sh
git -C /path/to/repo fetch origin
sy new feature /path/to/repo --start-point origin/main
```

For large repositories, select only directories required by the task:

```sh
sy new feature /path/to/repo --start-point origin/main \
  --sparse-directory src --sparse-directory tests
```

This creates a worktree without a full checkout, then populates the selected cone. Root files are included.
Directory names must exist in the selected commit. Settings stay local to that worktree.
Existing full checkouts are unchanged. `git sparse-checkout disable` restores the full checkout when needed.

## Claude Code and Codex

Start terminal harnesses inside a seshy-owned checkout when seshy owns the task.
When a desktop app creates a worktree, leave creation, handoff, and deletion with that app.
Do not create another worktree inside it for the same task. Use separate checkouts only for concurrent writers or a distinct task.

Seshy can group an existing app-owned worktree without copying it:

```sh
sy new investigation --empty
sy add investigation /path/to/app-worktree --reference
```

A reference shares the original files. It does not prevent the app from deleting its checkout and does not make parallel edits isolated.
Use the app's permanent-worktree or handoff feature for work that needs a different lifetime.
Replacing native creation hooks is unnecessary for inventory and can change setup-file copying and cleanup semantics.

## Inspect before cleanup

```sh
sy worktrees /path/to/repo --format json
sy worktrees /path/to/repo --disk-usage --format json
sy prune /path/to/repo --metadata-only --dry-run
```

Inventory uses Git registrations and deduplicates by common Git directory. It finds linked checkouts created by any harness.
Without repository arguments, it starts from active and archived seshy members plus the current repository.
This is not a whole-machine repository search. Pass an unrelated repository explicitly to inspect its worktrees.

Disk usage reports shared Git storage separately. Checkout measurements exclude registered nested worktrees and do not follow symlinks.
Hardlinks within a measured tree are counted once. Cross-tree hardlinks and filesystem clones can still share space.
Do not sum these numbers as guaranteed reclaimed space.

Dirty state includes untracked files but excludes ignored files. Ahead/behind uses existing local upstream references; inventory does not fetch.
A missing directory, clean checkout, or old timestamp does not establish permission to delete work.
Check ignored recovery files, unpublished commits, running processes, and the owning harness before removing a checkout.
Detached HEAD worktrees need particular care because a branch might not preserve their commits.

Metadata-only pruning preserves branches and references. It follows Git's normal expiry and lock rules and reports verified removals after applying.
Archive preserves the workspace and does not reclaim disk space. Use explicit reviewed removal only after the work is retained elsewhere.

## What not to share

Use shared package download stores and compiler caches when the tool supports them.
Keep mutable installations and build output separate between branches. Sharing `node_modules` or virtual environments by symlink can break isolation.
For a new large repository, partial clone can reduce downloaded history blobs. It does not shrink an existing checkout automatically.
Object alternates introduce a dependency on another clone's retained objects. They add little value when linked worktrees already share history.

References: [Git worktrees](https://git-scm.com/docs/git-worktree),
[partial clone and reference clones](https://git-scm.com/docs/git-clone),
[sparse checkout](https://git-scm.com/docs/git-sparse-checkout),
[Codex worktrees](https://learn.chatgpt.com/docs/environments/git-worktrees),
[Claude Code hooks](https://code.claude.com/docs/en/hooks#worktreecreate).
