# wezterm

Open Seshy sessions from the WezTerm picker.

## Install

```sh
brew install roshbhatia/tap/seshy-provider-wezterm
nix profile add 'github:roshbhatia/seshy#provider-wezterm'
```

Install the core utility separately, or select its all-provider bundle. Runtime tools still need their own credentials.

The provider advertises `picker.create`. Choose `New session`, enter a valid Seshy name, then select repositories in the terminal.
Sessions persist through Seshy's existing storage. The adapter also advertises
`picker.rename` and `picker.delete`, using `sy rename` and `sy delete --yes` after
the picker confirms deletion. Deletion removes the session and its owned
worktrees, preserves branches, and retains the normal dirty-worktree safety
checks. Provider errors include the CLI's refusal reason.
The helper runs `sy new` and starts WezTerm's configured shell in the new session.
Cancelling repository selection closes the temporary terminal. Existing sessions are never replaced.

## Demo

Recording pending. The previous script printed adapter output without exercising WezTerm.

[Tape source](demo.tape)
