package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/roshbhatia/seshy/internal/config"
	"github.com/roshbhatia/seshy/internal/hook"
	"github.com/roshbhatia/seshy/internal/session"
	"github.com/roshbhatia/seshy/internal/ui"
	"github.com/spf13/cobra"
)

var forceDelete bool

var deleteCmd = &cobra.Command{
	Use:     "delete [name]",
	Short:   "Delete a session",
	Aliases: []string{"rm"},
	Args:    cobra.MaximumNArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		sessions, _ := session.List()
		names := make([]string, len(sessions))
		for i, s := range sessions {
			names[i] = s.Name
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		var name string

		if len(args) > 0 {
			name = args[0]
		} else {
			if forceDelete {
				return fmt.Errorf("--force requires a session name argument")
			}
			sessions, err := session.List()
			if err != nil {
				return fmt.Errorf("listing sessions: %w", err)
			}
			if len(sessions) == 0 {
				fmt.Fprintln(os.Stderr, ui.Info("No sessions to delete."))
				return nil
			}
			names := make([]string, len(sessions))
			for i, s := range sessions {
				names[i] = s.Name
			}
			selected, err := runPicker(cfg.SessionPicker, names)
			if err != nil {
				if errors.Is(err, errCancelled) {
					return nil
				}
				return err
			}
			if len(selected) == 0 {
				return fmt.Errorf("no session selected")
			}
			name = selected[0]
		}

		if !session.Exists(name) {
			return fmt.Errorf("session %s not found", ui.AccentBold(name))
		}

		if !forceDelete {
			fmt.Fprintf(os.Stderr, "Delete session %s and its worktrees/branches? [y/N] ", ui.AccentBold(name))
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Fprintln(os.Stderr, ui.Info("Cancelled."))
				return nil
			}
		}

		// Run pre-delete hooks with full repo info
		sessionPath, _ := session.GetPath(name)
		repoInfos := session.GetSessionRepoInfos(sessionPath)
		data := session.BuildTemplateData(name, sessionPath, repoInfos)
		hook.Run("pre-delete", cfg.Hooks.PreDelete, data, sessionPath)

		if err := session.Delete(name); err != nil {
			return fmt.Errorf("failed to delete session: %w", err)
		}

		fmt.Fprintln(os.Stderr, ui.Successf("Deleted session %s", ui.AccentBold(name)))
		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolVarP(&forceDelete, "force", "f", false, "Skip confirmation prompt")
	rootCmd.AddCommand(deleteCmd)
}
