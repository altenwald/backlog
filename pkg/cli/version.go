package cli

import (
	"fmt"

	"github.com/altenwald/backlog/pkg/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print Backlog version, commit hash and build date",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.FullInfo())
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
	RootCmd.Version = version.String()
}
