# Seshy agent context

`sy` owns grouped Git workspaces and worktree lifecycle.

- `cmd/` owns Cobra commands and generated CLI metadata; `cmd/sy` is the entry point.
- `internal/session` owns session records and worktrees.
- `internal/config` owns settings; `internal/hook` owns lifecycle hooks.
- `extras/wezterm` owns optional terminal integration.
- `docs/worktree-lifecycle.md` defines ownership and storage behavior.

Inventory, dry-run, archive, metadata pruning, and deletion are distinct actions.
Preserve external lifecycle ownership and dirty/unpublished work. A clean Git
status does not prove ignored files or running sessions are disposable. Use
isolated temporary repositories and XDG state in tests, never live sessions.

## Working locally

Inspect active changes and preserve unrelated work, sessions, and build roots.
Use `nix develop` for dependencies and edit Nix sources for managed settings.
Keep documented stdout/JSON contracts stable and diagnostics on stderr. Reuse
existing `go-utils` contracts; keep optional integrations outside the core.
Regenerate derived artifacts with their owning command and inspect the full diff.

When sysinit's managed skills are available, use `search-code-routing`,
`debugging`, and `change-validation` for their respective workflows. Durable work
uses `local-workflows` and `taskwarrior`; worktree creation and cleanup use
`feature-based-session-manager`. Taskwarrior owns tasks, `sy` owns workspaces,
`task-queue` owns explicit Pueue attempts, and `worker` owns requested visible
WezTerm runs. Keep ordinary checks here. Tool availability does not authorize
additional agents, queued work, session interruption, or external messages.
Downstream pins and host activation remain separate operations.

## Checks

```sh
nix develop
go vet ./...
go test -race ./...
bash hack/test-screenshots.sh
go run ./cmd/sy generate --check
bash test/integration/run_tests.sh
bash test/e2e/run_e2e.sh
nix fmt -- --check
nix flake check
```

Use the owning `hack/update-*` command for dependency and vendor-hash updates.
Tagged releases must match package metadata and pass repository checks. Changes
to providers also require their affected `extras/` checks.

For changes that do not affect rendered behavior, `hack/screenshots.sh --refresh-fingerprint` validates existing images and refreshes their metadata.
Regenerate media with `hack/screenshots.sh` when rendered behavior changes.
