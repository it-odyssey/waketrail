package cmd

import (
	"encoding/json"

	gitcollector "github.com/it-odyssey/waketrail/internal/collectors/git"
	"github.com/it-odyssey/waketrail/internal/localdata"
	"github.com/spf13/cobra"
)

// The Bash hook invokes this immediately before eligible Git commands. Its
// temporary output is metadata-only and removed after the matching record.
var gitSnapshotCmd = &cobra.Command{
	Use: "git-snapshot", Hidden: true, Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := cmd.Flags().GetString("cwd")
		output, _ := cmd.Flags().GetString("output")
		snapshot, err := gitcollector.Detect(cwd)
		if err != nil {
			return err
		}
		data, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		return localdata.WritePrivate(output, data)
	},
}

func init() {
	gitSnapshotCmd.Flags().String("cwd", "", "working directory")
	gitSnapshotCmd.Flags().String("output", "", "snapshot file")
	_ = gitSnapshotCmd.MarkFlagRequired("cwd")
	_ = gitSnapshotCmd.MarkFlagRequired("output")
	rootCmd.AddCommand(gitSnapshotCmd)
}
