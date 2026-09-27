package cmd

import (
	"fmt"

	"github.com/it-odyssey/waketrail/internal/capture"
	"github.com/spf13/cobra"
)

var classifyCmd = &cobra.Command{
	Use:    "classify [command]",
	Short:  "Classify a command for output capture",
	Hidden: true,
	Args:   cobra.ExactArgs(1),

	RunE: func(cmd *cobra.Command, args []string) error {
		mode := capture.Classify(args[0])

		fmt.Fprintln(cmd.OutOrStdout(), mode)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(classifyCmd)
}
