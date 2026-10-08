package cmd

import (
	"errors"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/it-odyssey/waketrail/internal/capture"
	"github.com/it-odyssey/waketrail/internal/localdata"
	"github.com/spf13/cobra"
)

// This internal tee replacement keeps terminal output intact while capture is
// bounded. Ctrl-C interrupts the user's command; the drainer survives long enough
// to finish its spool when the hook restores descriptors at the next prompt.
var captureStreamCmd = &cobra.Command{
	Use: "capture-stream", Hidden: true,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		signal.Ignore(os.Interrupt, syscall.SIGHUP)
		path, err := cmd.Flags().GetString("output")
		if err != nil {
			return err
		}
		file, err := localdata.OpenPrivate(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
		if err != nil {
			// Capture problems must not discard the command's live output.
			_, copyErr := io.Copy(cmd.OutOrStdout(), cmd.InOrStdin())
			return errors.Join(err, copyErr)
		}
		streamErr := capture.Stream(cmd.InOrStdin(), cmd.OutOrStdout(), file)
		return errors.Join(streamErr, file.Close())
	},
}

func init() {
	captureStreamCmd.Flags().String("output", "", "Private capture spool path")
	rootCmd.AddCommand(captureStreamCmd)
}
