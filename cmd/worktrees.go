package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/roshbhatia/seshy/internal/session"
	"github.com/spf13/cobra"
)

var worktreesFormat string
var worktreesDiskUsage bool

var worktreesCmd = &cobra.Command{
	Use:   "worktrees [repo...]",
	Short: "Inventory all registered worktrees, including external harness checkouts",
	Long: `Inventory Git registrations without moving or adopting worktrees.

Without repo arguments, inspect the current repository and repositories linked
from active and archived sessions. Explicit arguments restrict that scope.
Git's common directory deduplicates linked checkouts from any harness.

--disk-usage measures allocated bytes without following symlinks. Shared Git
storage is separate; registered nested worktrees are excluded from their parent's
checkout size. APFS clones and hardlinks across checkouts can still share blocks.
This scan is opt-in because build and dependency directories can be large.

Status includes untracked files, but excludes ignored files. Ahead/behind uses
local upstream refs and does not fetch. Cautions are a review preview, never
permission to delete: active processes and harness retention are not checked.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, err := resolveFormat(worktreesFormat, nil, formatTable, formatJSON)
		if err != nil {
			return err
		}
		repos, _ := readRepoArgs(args, false, os.Stdin)
		report, inspectErr := session.Inventory(repos, worktreesDiskUsage)
		if format == formatJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(report); err != nil {
				return err
			}
		} else {
			rows := [][]string{}
			for _, repo := range report.Repositories {
				if repo.SharedGitBytes != nil {
					rows = append(rows, []string{contractHome(repo.CommonDir), "(shared Git)", "", fmt.Sprintf("%.1f", float64(*repo.SharedGitBytes)/(1024*1024)), "counted once per repository"})
				}
				for _, tree := range repo.Worktrees {
					size := "-"
					if tree.CheckoutBytes != nil {
						size = fmt.Sprintf("%.1f", float64(*tree.CheckoutBytes)/(1024*1024))
					}
					branch := tree.Branch
					if tree.Detached {
						branch = "(detached)"
					}
					if tree.Bare {
						branch = "(bare)"
					}
					rows = append(rows, []string{contractHome(tree.Path), branch, strings.Join(tree.Sessions, ","), size, strings.Join(tree.Cautions, "; ")})
				}
			}
			if err := table(os.Stdout, []string{"PATH", "BRANCH", "SESSIONS", "MiB", "CAUTIONS"}, rows); err != nil {
				return err
			}
		}
		return inspectErr
	},
}

func init() {
	worktreesCmd.Flags().StringVar(&worktreesFormat, "format", "", "Output format: table, json")
	worktreesCmd.Flags().BoolVar(&worktreesDiskUsage, "disk-usage", false, "Measure shared Git and checkout allocated bytes")
	rootCmd.AddCommand(worktreesCmd)
}
