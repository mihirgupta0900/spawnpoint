package cli

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/mihirgupta0900/spawnpoint/internal/ui"
	"github.com/mihirgupta0900/spawnpoint/internal/update"
)

func updateCmd() *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update spawnpoint (uses brew, pipx, uv or pip if that's how you installed it)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			current := cmd.Root().Version
			m := update.Detect()

			var latest string
			ui.Spin("Checking for updates…", func() {
				latest, _ = update.Latest()
			})
			ui.Println(ui.Dim("installed ") + current + ui.Dim("  via ") + m.Name)
			if latest == "" {
				return fail("Couldn't reach GitHub to check the latest release.")
			}
			ui.Println(ui.Dim("latest    ") + latest)

			// Dev builds (unparseable versions) always count as outdated.
			outdated := update.Newer(latest, current) || !update.Newer(current, "0.0.0")
			if !outdated {
				ui.Success("Already up to date.")
				return nil
			}
			if check {
				ui.Println(ui.Warn("Update available.") + " Run " + ui.Bold("sp update") + ui.Dim("  ("+m.Display()+")"))
				return nil
			}

			if m.Cmd != nil {
				ui.Println(ui.Dim("running ") + ui.Bold(m.Display()))
				c := exec.Command(m.Cmd[0], m.Cmd[1:]...)
				c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stderr, os.Stderr
				if err := c.Run(); err != nil {
					return fail("%s failed: %v", m.Display(), err)
				}
				ui.Success("Updated to %s", latest)
				return nil
			}

			var err error
			ui.Spin("Downloading "+latest+"…", func() { err = update.SelfUpdate(latest, m.Exe) })
			if err != nil {
				return fail("%v", err)
			}
			ui.Success("Updated %s → %s", current, latest)
			return nil
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "only report whether an update is available")
	return cmd
}
