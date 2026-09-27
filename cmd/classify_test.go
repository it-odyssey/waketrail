package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestClassifyCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected string
	}{
		{
			name:     "bounded command",
			command:  "docker logs nginx",
			expected: "bounded",
		},
		{
			name:     "output command",
			command:  "git status",
			expected: "output",
		},
		{
			name:     "metadata only command",
			command:  "docker stop nginx",
			expected: "none",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer

			classifyCmd.SetOut(&output)

			err := classifyCmd.RunE(
				classifyCmd,
				[]string{test.command},
			)
			if err != nil {
				t.Fatalf(
					"classifyCmd.RunE() returned error: %v",
					err,
				)
			}

			got := strings.TrimSpace(output.String())

			if got != test.expected {
				t.Errorf(
					"output = %q, want %q",
					got,
					test.expected,
				)
			}
		})
	}
}
