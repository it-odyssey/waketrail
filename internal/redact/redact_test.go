package redact

import "testing"

func TestStringRedactsSensitiveValues(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "token assignment",
			input:    "TOKEN=supersecret123",
			expected: "TOKEN=[REDACTED]",
		},
		{
			name:     "password assignment",
			input:    "password=hunter2",
			expected: "password=[REDACTED]",
		},
		{
			name:     "token flag",
			input:    "tool login --token abc123",
			expected: "tool login --token [REDACTED]",
		},
		{
			name:     "password flag",
			input:    "tool login --password hunter2",
			expected: "tool login --password [REDACTED]",
		},
		{
			name:     "bearer token",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9",
			expected: "Authorization: Bearer [REDACTED]",
		},
		{
			name:     "aws access key",
			input:    "AKIAIOSFODNN7EXAMPLE",
			expected: "[REDACTED]",
		},
		{
			name:     "aws secret access key",
			input:    "aws_secret_access_key=abcdefghijklmnopqrstuvwxyz1234567890ABCD",
			expected: "aws_secret_access_key=[REDACTED]",
		},
		{
			name:     "ordinary text untouched",
			input:    "docker ps",
			expected: "docker ps",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := String(test.input)

			if actual != test.expected {
				t.Fatalf(
					"String(%q) = %q, want %q",
					test.input,
					actual,
					test.expected,
				)
			}
		})
	}
}
