package capture

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/it-odyssey/waketrail/internal/redact"
)

// OutputLimit is the maximum persisted bytes per stream, including markers.
// All eligible capture modes share it; bounded no longer means a different cap.
const OutputLimit = 64 * 1024

const omissionMarker = "\n[... output omitted during capture ...]\n"
const privateTailMarker = "\n[... output omitted; tail withheld to preserve redaction context ...]\n"

// LimitOutput sanitizes before shortening an in-memory value. Complete lines
// prevent truncation from manufacturing fragments of credentials or UTF-8 text.
func LimitOutput(value string) (string, bool) {
	value = redact.String(value)
	if len(value) <= OutputLimit {
		return value, false
	}
	head, tail := outputSections(value, omissionMarker)
	return head + omissionMarker + tail, true
}

func outputSections(value, marker string) (string, string) {
	headBudget := OutputLimit / 4
	tailBudget := OutputLimit - headBudget - len(marker)
	return completeHead(value[:headBudget]), completeTail(value[len(value)-tailBudget:])
}

func completeHead(value string) string {
	if i := strings.LastIndexByte(value, '\n'); i >= 0 {
		return value[:i+1]
	}
	return ""
}

func completeTail(value string) string {
	if i := strings.IndexByte(value, '\n'); i >= 0 {
		return value[i+1:]
	}
	return ""
}

// ReadOutput reads regular capture files with bounded memory. It retains about
// 16 KiB of head and 48 KiB of tail, counts original bytes, and redacts before
// returning text to the recorder. A truncated secret-bearing stream is head-only:
// arbitrary tail fragments can lack the PEM/YAML/JSON label needed for redaction.
func ReadOutput(path string) (string, int64, bool, error) {
	if path == "" {
		return "", 0, false, nil
	}
	// Check before opening, because opening a FIFO can block before Stat runs.
	info, err := os.Stat(path)
	if err != nil {
		return "", 0, false, err
	}
	if !info.Mode().IsRegular() {
		return "", 0, false, fmt.Errorf("capture input must be a regular file: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", 0, false, err
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil {
		return "", 0, false, err
	}
	if !info.Mode().IsRegular() {
		return "", 0, false, fmt.Errorf("capture input must be a regular file: %s", path)
	}
	total := info.Size()
	if total <= OutputLimit {
		// LimitReader also bounds reads if another process grows the file.
		data, err := io.ReadAll(io.LimitReader(file, OutputLimit))
		if err != nil {
			return "", 0, false, err
		}
		text, shortened := LimitOutput(string(data))
		return text, total, shortened, nil
	}

	headData := make([]byte, OutputLimit/4)
	if _, err := io.ReadFull(file, headData); err != nil {
		return "", 0, false, err
	}
	head := redact.String(completeHead(string(headData)))
	// Inspect overlapping fixed-size windows, including the omitted region.
	// This is intentionally conservative: any recognized secret anywhere in an
	// oversized stream withholds its tail instead of losing redaction context.
	secret, err := containsRedaction(file, total)
	if err != nil {
		return "", 0, false, err
	}
	if secret {
		text, _ := LimitOutput(head + privateTailMarker)
		return text, total, true, nil
	}
	tailData := make([]byte, OutputLimit-OutputLimit/4-len(omissionMarker))
	if _, err := file.ReadAt(tailData, total-int64(len(tailData))); err != nil {
		return "", 0, false, err
	}
	text, _ := LimitOutput(head + omissionMarker + completeTail(string(tailData)))
	return text, total, true, nil
}

func containsRedaction(file *os.File, total int64) (bool, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return false, err
	}
	reader := io.LimitReader(file, total)
	buffer := make([]byte, OutputLimit)
	previous := ""
	for {
		n, err := io.ReadFull(reader, buffer)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return false, err
		}
		chunk := string(buffer[:n])
		window := previous + chunk
		if redact.String(window) != window {
			return true, nil
		}
		if err != nil {
			return false, nil
		}
		previous = chunk
	}
}
