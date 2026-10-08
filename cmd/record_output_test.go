package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/it-odyssey/waketrail/internal/capture"
)

func TestCapturedTailPreservesTerraformSummaries(t *testing.T) {
	for _, tc := range []struct{ command, summary, event string }{
		{"terraform plan", "Plan: 2 to add, 1 to change, 0 to destroy.", "plan"},
		{"terraform apply", "Apply complete! Resources: 2 added, 1 changed, 0 destroyed.", "apply"},
		{"tofu destroy", "Destroy complete! Resources: 2 destroyed.", "destroy"},
	} {
		t.Run(tc.event, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "stdout")
			value := "Initial diagnostic evidence\n" + strings.Repeat("ordinary diagnostic line\n", 10000) + "\x1b[32m" + tc.summary + "\x1b[0m\n"
			if err := os.WriteFile(path, []byte(value), 0600); err != nil {
				t.Fatal(err)
			}
			output, err := loadCommandOutput(1, path, path)
			if err != nil {
				t.Fatal(err)
			}
			if !output.StdoutTruncated || !output.StderrTruncated || output.StdoutBytes != int64(len(value)) || output.StderrBytes != int64(len(value)) || len(output.Stdout) > capture.OutputLimit || len(output.Stderr) > capture.OutputLimit {
				t.Fatal("stream limits or original counts incorrect")
			}
			event, ok := terraformTimelineEvent(tc.command, "/tmp/infra", output.Stdout, "", 1, time.Now())
			if !ok || event.EventType != tc.event || event.Resource != "infra" {
				t.Fatalf("Terraform tail did not parse: %+v %v", event, ok)
			}
		})
	}
}
