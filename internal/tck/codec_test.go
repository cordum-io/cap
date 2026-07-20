package tck

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestDecodeReadsHandshakeAndResult(t *testing.T) {
	in := strings.Join([]string{
		`{"type":"handshake","protocolVersion":1,"role":"worker","sdk":"go","transport":"nats"}`,
		`{"type":"result","id":"c1","status":"PASS","evidence":{"subject":"sys.job.result"}}`,
	}, "\n")
	d := NewDecoder(strings.NewReader(in))

	hs, err := d.Decode()
	if err != nil {
		t.Fatalf("decode handshake: %v", err)
	}
	if hs.Type != MsgHandshake || hs.Role != RoleWorker || hs.SDK != "go" {
		t.Fatalf("unexpected handshake: %+v", hs)
	}

	res, err := d.Decode()
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if res.ID != "c1" || res.Status != StatusPass || res.Evidence["subject"] != "sys.job.result" {
		t.Fatalf("unexpected result: %+v", res)
	}

	if _, err := d.Decode(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected io.EOF at end, got %v", err)
	}
}

func TestDecodeSkipsBlankLines(t *testing.T) {
	in := "\n   \n" + `{"type":"result","id":"c1","status":"FAIL"}` + "\n\n"
	d := NewDecoder(strings.NewReader(in))
	msg, err := d.Decode()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if msg.Status != StatusFail {
		t.Fatalf("expected FAIL, got %+v", msg)
	}
	if _, err := d.Decode(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected io.EOF, got %v", err)
	}
}

func TestDecodeRejectsMalformedJSON(t *testing.T) {
	d := NewDecoder(strings.NewReader("{not json}\n"))
	if _, err := d.Decode(); err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

// adapter-v1 is exactly one JSON object per line. Two concatenated objects on a
// single line must be refused, not silently accepted as the first — otherwise a
// malformed or duplicate result could ride along unread after the first object.
func TestDecodeRejectsTrailingData(t *testing.T) {
	line := `{"type":"result","id":"c1","status":"PASS"}{"type":"result","id":"c2","status":"FAIL"}`
	d := NewDecoder(strings.NewReader(line + "\n"))
	if _, err := d.Decode(); err == nil {
		t.Fatal("expected error for trailing data after adapter message, got nil")
	}
}

// A single object followed only by whitespace is still valid and must decode.
func TestDecodeAllowsTrailingWhitespace(t *testing.T) {
	d := NewDecoder(strings.NewReader(`{"type":"result","id":"c1","status":"PASS"}   ` + "\n"))
	msg, err := d.Decode()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if msg.ID != "c1" || msg.Status != StatusPass {
		t.Fatalf("unexpected message: %+v", msg)
	}
}

// An unknown field is a contract drift the harness must not silently ignore.
func TestDecodeRejectsUnknownField(t *testing.T) {
	d := NewDecoder(strings.NewReader(`{"type":"result","surprise":true}` + "\n"))
	if _, err := d.Decode(); err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}

// A hostile adapter emitting a line larger than the bound must be refused with
// a specific error, not buffered without limit.
func TestDecodeRejectsOverlongLine(t *testing.T) {
	huge := `{"type":"result","detail":"` + strings.Repeat("x", MaxLineBytes) + `"}`
	d := NewDecoder(strings.NewReader(huge + "\n"))
	if _, err := d.Decode(); !errors.Is(err, ErrLineTooLong) {
		t.Fatalf("expected ErrLineTooLong, got %v", err)
	}
}

func TestEncodeRoundTrips(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	cmd := Command{Type: MsgRun, ID: "c9", Scenario: "lifecycle/happy", Params: map[string]string{"attempt": "1"}}
	if err := enc.Encode(cmd); err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !bytes.HasSuffix(buf.Bytes(), []byte("\n")) {
		t.Fatal("encoded command must end with a newline")
	}
	// A runner->adapter command decodes on the adapter side; assert the wire
	// carries exactly the fields we set.
	got := buf.String()
	for _, want := range []string{`"type":"run"`, `"id":"c9"`, `"scenario":"lifecycle/happy"`, `"attempt":"1"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("encoded command missing %s: %s", want, got)
		}
	}
}

func TestEncodeRejectsOverlongCommand(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	cmd := Command{Type: MsgRun, Params: map[string]string{"blob": strings.Repeat("y", MaxLineBytes)}}
	if err := enc.Encode(cmd); !errors.Is(err, ErrLineTooLong) {
		t.Fatalf("expected ErrLineTooLong, got %v", err)
	}
}

func TestIsRequiredNonPass(t *testing.T) {
	// For a required, applicable case only PASS satisfies it; an adapter that
	// reports N/A for a case it was asked to run has declined proof and fails.
	cases := map[Status]bool{
		StatusPass:        false,
		StatusNA:          true,
		StatusFail:        true,
		StatusError:       true,
		StatusUnsupported: true,
	}
	for s, want := range cases {
		if got := IsRequiredNonPass(s); got != want {
			t.Fatalf("IsRequiredNonPass(%s)=%v want %v", s, got, want)
		}
	}
}
