package capture

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/it-odyssey/waketrail/internal/redact"
)

// The private spool carries sanitized text plus original byte/truncation metadata.
// Raw output stays in bounded process memory, never an ever-growing tee file.
const spoolMagic = "WakeTrail capture v1\n"
const spoolHeaderSize = len(spoolMagic) + 9
const SpoolLimit = spoolHeaderSize + OutputLimit

type streamSample struct {
	bytes    int64
	head     []byte
	tail     []byte
	pending  []byte
	previous string
	secret   bool
}

func (s *streamSample) Write(data []byte) (int, error) {
	length := len(data)
	s.bytes += int64(len(data))
	if len(s.head) < OutputLimit {
		n := min(len(data), OutputLimit-len(s.head))
		s.head = append(s.head, data[:n]...)
	}
	if len(data) >= OutputLimit {
		s.tail = append(s.tail[:0], data[len(data)-OutputLimit:]...)
	} else {
		if excess := len(s.tail) + len(data) - OutputLimit; excess > 0 {
			copy(s.tail, s.tail[excess:])
			s.tail = s.tail[:len(s.tail)-excess]
		}
		s.tail = append(s.tail, data...)
	}
	// Scan adjacent 64 KiB windows before discarded bytes leave memory. Once
	// a secret is recognized, the tail will be withheld and further scans stop.
	if !s.secret {
		for len(data) > 0 {
			n := min(len(data), OutputLimit-len(s.pending))
			s.pending = append(s.pending, data[:n]...)
			data = data[n:]
			if len(s.pending) == OutputLimit {
				s.inspect()
				if s.secret {
					break
				}
			}
		}
	}
	return length, nil
}

func (s *streamSample) inspect() {
	chunk := string(s.pending)
	window := s.previous + chunk
	s.secret = redact.String(window) != window
	s.previous = chunk
	s.pending = s.pending[:0]
}

func (s *streamSample) result() (string, bool) {
	if s.bytes <= OutputLimit {
		return LimitOutput(string(s.head))
	}
	if !s.secret {
		s.inspect()
	}
	head := redact.String(completeHead(string(s.head[:OutputLimit/4])))
	if s.secret {
		text, _ := LimitOutput(head + privateTailMarker)
		return text, true
	}
	tailBudget := OutputLimit - OutputLimit/4 - len(omissionMarker)
	tail := completeTail(string(s.tail[len(s.tail)-tailBudget:]))
	text, _ := LimitOutput(head + omissionMarker + tail)
	return text, true
}

// Stream forwards output unchanged and writes one capped sanitized spool at EOF.
// Capture-file failures are reported only after forwarding has completed.
func Stream(input io.Reader, live io.Writer, spool io.Writer) error {
	sample := &streamSample{}
	_, copyErr := io.Copy(live, io.TeeReader(input, sample))
	text, truncated := sample.result()
	header := make([]byte, spoolHeaderSize)
	copy(header, spoolMagic)
	binary.BigEndian.PutUint64(header[len(spoolMagic):], uint64(sample.bytes))
	if truncated {
		header[len(header)-1] = 1
	}
	n, headerErr := spool.Write(header)
	if headerErr == nil && n != len(header) {
		headerErr = io.ErrShortWrite
	}
	var textErr error
	if headerErr == nil {
		n, textErr = io.WriteString(spool, text)
		if textErr == nil && n != len(text) {
			textErr = io.ErrShortWrite
		}
	}
	return errors.Join(copyErr, headerErr, textErr)
}

// readSpool distinguishes new internal spools from legacy plain capture files.
// Invalid envelopes fail explicitly rather than being treated as raw evidence.
func readSpool(file *os.File, size int64) (string, int64, bool, bool, error) {
	magic := make([]byte, len(spoolMagic))
	if n, _ := file.ReadAt(magic, 0); n != len(magic) || string(magic) != spoolMagic {
		return "", 0, false, false, nil
	}
	if size < int64(spoolHeaderSize) || size > int64(SpoolLimit) {
		return "", 0, false, true, fmt.Errorf("invalid capture spool size")
	}
	data := make([]byte, size)
	if _, err := file.ReadAt(data, 0); err != nil {
		return "", 0, false, true, err
	}
	count := binary.BigEndian.Uint64(data[len(spoolMagic):])
	flag := data[spoolHeaderSize-1]
	if count > uint64(1<<63-1) || flag > 1 || (count > OutputLimit && flag == 0) {
		return "", 0, false, true, fmt.Errorf("invalid capture spool metadata")
	}
	text, shortened := LimitOutput(string(data[spoolHeaderSize:]))
	return text, int64(count), flag == 1 || shortened, true, nil
}
