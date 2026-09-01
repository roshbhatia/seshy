# seshy

Minimalist session manager for multi-repo development, with git worktree integration.

## Install

```bash
nix run github:roshbhatia/seshy -- --help
```

Tagged releases also publish `sy` archives for Darwin and Linux.

## Commands

| Command | Description |
|---------|-------------|
| `sy new <name>` | Create a new session, selecting repos from zoxide history |
| `sy new <name> --empty` | Create an empty session with no repositories |
| `sy add <name>` | Add repositories to an existing session |
| `sy list` / `sy ls` | List all sessions |
| `sy current` | Print the session holding the working directory |
| `sy switch <name>` | Resolve a session and print its path |
| `sy attach <name>` | Print session metadata as JSON |
| `sy list --archived` | List archived sessions |
| `sy archive <name>` | Move a session into the archive |
| `sy unarchive <name>` / `sy restore <name>` | Restore an archived session |
| `sy delete <name>` / `sy rm <name>` | Delete a session and clean up worktrees + branches |
| `sy delete --archived <name>` | Delete an archived session |
| `sy path <name>` | Print session path |
| `sy config` | Show effective configuration |
| `sy config edit` | Open config file in editor |
| `sy --greedy <query>` | Fuzzy match session and print its path |

## Configuration

Config file at `~/.config/seshy/config.yaml` (or `$XDG_CONFIG_HOME/seshy/config.yaml`):

```yaml
# Branch naming template. Variables: {{.Session}}, {{.Repo}}, {{.User}}
branchFormat: "sy/{{.Session}}/{{.Repo}}"

# Sessions storage directory
sessionsDir: "~/.local/state/seshy/sessions"

# Archive storage directory. Defaults to a sibling of sessionsDir, so archiving
# stays a same-filesystem move.
archiveDir: "~/.local/state/seshy/archive"
```

Run `sy config` to see effective settings, `sy config edit` to modify.

## Archiving

Archiving moves a session out of the way without tearing it down. Worktrees,
branches, and uncommitted work all survive the move, and the main repos are
repaired so they track the new location.

```bash
sy archive my-feature      # move it to ~/.local/state/seshy/archive
sy list --archived         # see what is archived
sy unarchive my-feature    # move it back
```

Neither command prompts for confirmation, because neither destroys anything.
Archived sessions do not appear in `sy list`. To throw one away for good, run
`sy delete --archived <name>`.

## Empty Sessions

A session does not need any repositories. Use `--empty` to create one with none:

```bash
sy new scratch --empty
sy add scratch          # attach repos later
```

Two other paths create an empty session instead of prompting:

- `sy new <name> --stdin` when stdin holds no paths.
- `sy new <name>` when the repo source returns no candidates.

Non-git directories are always supported. Seshy symlinks them into the session
instead of creating a worktree, and `sy status` marks them `(symlink)`.

## Branch Naming

By default, worktree branches are named `sy/<session>/<repo>`. Override per-invocation:

```bash
sy new my-feature --branch hotfix/urgent
sy add my-feature -b feature/custom-branch
```

Or set a custom template in config:

```yaml
branchFormat: "dev/{{.User}}/{{.Session}}/{{.Repo}}"
```
