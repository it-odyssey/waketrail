package capture

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fragmentReader struct {
	reader io.Reader
	size   int
}

func (r fragmentReader) Read(data []byte) (int, error) {
	return r.reader.Read(data[:min(len(data), r.size)])
}

func TestStreamForwardsBytesAndCapsSpool(t *testing.T) {
	for _, value := range []string{
		"", "TOKEN=a\nordinary output\n", strings.Repeat("x", OutputLimit),
		"HEAD\n" + strings.Repeat("ordinary output\n", 10000) + "TAIL\n",
		"HEAD\n" + strings.Repeat("ordinary output\n", 5000) + "password: |\n" + strings.Repeat("  synthetic-secret\n", 10000),
	} {
		for _, fragmentSize := range []int{7, 32768} {
			path := filepath.Join(t.TempDir(), "spool")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			var live bytes.Buffer
			if err := Stream(fragmentReader{strings.NewReader(value), fragmentSize}, &live, file); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			if live.String() != value {
				t.Fatal("live output changed")
			}
			info, err := os.Stat(path)
			if err != nil || info.Size() > int64(SpoolLimit) {
				t.Fatal("spool exceeded cap")
			}
			actual, count, truncated, err := ReadOutput(path)
			if err != nil || count != int64(len(value)) || len(actual) > OutputLimit || truncated != (len(value) > OutputLimit) || strings.Contains(actual, "synthetic-secret") || strings.Contains(actual, "TOKEN=a") {
				t.Fatalf("fragment=%d: metadata/redaction failed: %d %v %v", fragmentSize, count, truncated, err)
			}
			// The streaming helper and legacy recorder must preserve the same
			// privacy exception, head/tail, counts and omission behavior.
			legacy, legacyCount, legacyTruncated, err := ReadOutput(writeOutput(t, value))
			if err != nil || actual != legacy || count != legacyCount || truncated != legacyTruncated {
				t.Fatal("streaming changed legacy capture semantics")
			}
		}
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, errors.New("capture failure") }

func TestStreamCaptureFailureKeepsLiveOutput(t *testing.T) {
	value := strings.Repeat("ordinary evidence\n", 10000)
	var live bytes.Buffer
	if err := Stream(strings.NewReader(value), &live, failedWriter{}); err == nil {
		t.Fatal("capture failure not reported")
	}
	if live.String() != value {
		t.Fatal("capture failure discarded live output")
	}
}

func TestReadOutputRejectsMalformedSpools(t *testing.T) {
	for _, value := range []string{spoolMagic, spoolMagic + strings.Repeat("x", SpoolLimit)} {
		if _, _, _, err := ReadOutput(writeOutput(t, value)); err == nil {
			t.Fatal("malformed spool accepted")
		}
	}
}
