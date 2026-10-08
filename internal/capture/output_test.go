package capture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeOutput(t *testing.T, value string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdout")
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadOutputSizeBoundaries(t *testing.T) {
	for _, size := range []int{0, OutputLimit - 1, OutputLimit, OutputLimit + 1, 8 * OutputLimit} {
		value := strings.Repeat("x", size)
		actual, bytes, truncated, err := ReadOutput(writeOutput(t, value))
		if err != nil {
			t.Fatal(err)
		}
		if bytes != int64(size) || truncated != (size > OutputLimit) || len(actual) > OutputLimit {
			t.Fatalf("size=%d: bytes=%d truncated=%v retained=%d", size, bytes, truncated, len(actual))
		}
		if size <= OutputLimit && actual != value {
			t.Fatal("untruncated evidence changed")
		}
		if size > OutputLimit && !strings.Contains(actual, "output omitted") {
			t.Fatal("missing omission marker")
		}
	}
}

func TestReadOutputKeepsHeadAndLargerTail(t *testing.T) {
	value := "INITIAL EVIDENCE\n" + strings.Repeat("ordinary diagnostic line\n", 10000) + "FINAL EVIDENCE\n"
	actual, bytes, truncated, err := ReadOutput(writeOutput(t, value))
	if err != nil {
		t.Fatal(err)
	}
	if bytes != int64(len(value)) || !truncated || len(actual) > OutputLimit {
		t.Fatal("incorrect metadata or cap")
	}
	head, tail, ok := strings.Cut(actual, omissionMarker)
	if !ok || !strings.HasPrefix(head, "INITIAL EVIDENCE") || !strings.Contains(tail, "FINAL EVIDENCE") || len(tail) <= len(head) {
		t.Fatal("useful head and larger tail not preserved")
	}
}

func TestReadOutputWithholdsSecretBearingTail(t *testing.T) {
	for _, secret := range []string{
		"password: |\n  synthetic-secret\n",
		"-----BEGIN PRIVATE KEY-----\nsynthetic-key-material\n",
		"{\"credentials\": {\n\"value\": \"synthetic-nested-secret\",\n",
		"TOKEN=" + strings.Repeat("s", 2*OutputLimit) + "\n",
	} {
		// The header lands across scan windows, while the tail lands inside
		// an unfinished block. Discarding the middle must not lose context.
		prefix := strings.Repeat("ordinary evidence\n", OutputLimit/18)
		value := prefix + secret + strings.Repeat("  synthetic-body\n", 12000)
		actual, bytes, truncated, err := ReadOutput(writeOutput(t, value))
		if err != nil {
			t.Fatal(err)
		}
		if !truncated || bytes != int64(len(value)) || len(actual) > OutputLimit || !strings.Contains(actual, "tail withheld") || strings.Contains(actual, "synthetic-") {
			t.Fatalf("unsafe truncation: %.200s", actual)
		}
	}
}

func TestReadOutputRedactsSmallStreamsAndRejectsDirectories(t *testing.T) {
	value := "TOKEN=synthetic-token\nordinary evidence\n"
	actual, bytes, truncated, err := ReadOutput(writeOutput(t, value))
	if err != nil || bytes != int64(len(value)) || truncated || strings.Contains(actual, "synthetic-token") || !strings.Contains(actual, "ordinary evidence") {
		t.Fatalf("redaction or metadata: %q %d %v %v", actual, bytes, truncated, err)
	}
	if _, _, _, err := ReadOutput(t.TempDir()); err == nil {
		t.Fatal("directory accepted as capture input")
	}
	if actual, bytes, truncated, err := ReadOutput(""); err != nil || actual != "" || bytes != 0 || truncated {
		t.Fatal("empty optional path failed")
	}
}

func TestLimitOutputRedactsBeforeTakingTail(t *testing.T) {
	value := "ordinary evidence\npassword: |\n" + strings.Repeat("  synthetic-secret\n", 10000) + "result: success\n"
	actual, _ := LimitOutput(value)
	if strings.Contains(actual, "synthetic-secret") || !strings.Contains(actual, "result: success") || len(actual) > OutputLimit {
		t.Fatal("in-memory truncation lost redaction context")
	}
}
