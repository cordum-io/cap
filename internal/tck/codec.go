package tck

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// MaxLineBytes bounds a single adapter-v1 message. A child that floods a
// gigabyte on one line must be refused, not buffered — an adapter is untrusted
// input to the harness.
const MaxLineBytes = 1 << 20 // 1 MiB

// ErrLineTooLong is returned when an inbound line exceeds MaxLineBytes.
var ErrLineTooLong = errors.New("tck: adapter message exceeds max line length")

// Decoder reads adapter->runner messages as line-delimited JSON with a hard
// per-line bound.
type Decoder struct {
	sc *bufio.Scanner
}

// NewDecoder wraps r with a bounded line scanner.
func NewDecoder(r io.Reader) *Decoder {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), MaxLineBytes)
	return &Decoder{sc: sc}
}

// Decode returns the next message. It reports io.EOF at end of stream,
// ErrLineTooLong for an over-length line, and a wrapped error for malformed
// JSON. A blank line is skipped rather than treated as an empty message.
func (d *Decoder) Decode() (AdapterMessage, error) {
	for {
		if !d.sc.Scan() {
			if err := d.sc.Err(); err != nil {
				if errors.Is(err, bufio.ErrTooLong) {
					return AdapterMessage{}, ErrLineTooLong
				}
				return AdapterMessage{}, err
			}
			return AdapterMessage{}, io.EOF
		}
		line := d.sc.Bytes()
		if len(trimSpace(line)) == 0 {
			continue
		}
		var msg AdapterMessage
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&msg); err != nil {
			return AdapterMessage{}, fmt.Errorf("tck: malformed adapter message: %w", err)
		}
		return msg, nil
	}
}

// Encoder writes runner->adapter commands as line-delimited JSON.
type Encoder struct {
	w io.Writer
}

// NewEncoder wraps w.
func NewEncoder(w io.Writer) *Encoder { return &Encoder{w: w} }

// Encode marshals cmd and writes it followed by a newline. It rejects a message
// that would itself exceed MaxLineBytes so the runner cannot emit a line its
// own decoder would refuse.
func (e *Encoder) Encode(cmd Command) error {
	b, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("tck: marshal command: %w", err)
	}
	if len(b)+1 > MaxLineBytes {
		return ErrLineTooLong
	}
	b = append(b, '\n')
	_, err = e.w.Write(b)
	return err
}

func trimSpace(b []byte) []byte {
	start := 0
	for start < len(b) && (b[start] == ' ' || b[start] == '\t' || b[start] == '\r' || b[start] == '\n') {
		start++
	}
	end := len(b)
	for end > start && (b[end-1] == ' ' || b[end-1] == '\t' || b[end-1] == '\r' || b[end-1] == '\n') {
		end--
	}
	return b[start:end]
}
