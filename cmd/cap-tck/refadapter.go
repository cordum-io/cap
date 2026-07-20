package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"io"
	"strconv"
	"time"

	"github.com/cordum-io/cap/v2/internal/tck"
)

// runRefAdapter is the built-in reference adapter. It speaks adapter-v1 and
// exists solely to exercise the harness end to end; it implements no CAP
// behavior and is NOT an independent conformant implementation. Each case's
// outcome is taken from the scenario's "status" param (default PASS), and an
// optional "sleepMs" param simulates a slow adapter.
func runRefAdapter(args []string, stdin io.Reader, stdout io.Writer) int {
	fs := flag.NewFlagSet("__adapter", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	role := fs.String("role", "worker", "adapter role: worker|control-plane")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	w := bufio.NewWriter(stdout)
	emit := func(m tck.AdapterMessage) {
		b, _ := json.Marshal(m)
		w.Write(b)
		w.WriteByte('\n')
		w.Flush()
	}
	sc := bufio.NewScanner(stdin)
	sc.Buffer(make([]byte, 0, 64*1024), tck.MaxLineBytes)

	if !readOneCommand(sc) { // consume hello
		return 0
	}
	emit(tck.AdapterMessage{
		Type: tck.MsgHandshake, ProtocolVersion: tck.ProtocolVersion,
		Role: tck.Role(*role), SDK: "reference", Transport: "none",
		Profiles: []string{"core"},
	})
	return refAdapterLoop(sc, emit)
}

func refAdapterLoop(sc *bufio.Scanner, emit func(tck.AdapterMessage)) int {
	for {
		cmd, ok := decodeCommand(sc)
		if !ok || cmd.Type == tck.MsgBye {
			return 0
		}
		if cmd.Type != tck.MsgRun {
			continue
		}
		if ms, err := strconv.Atoi(cmd.Params["sleepMs"]); err == nil && ms > 0 {
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}
		status := tck.Status(cmd.Params["status"])
		if status == "" {
			status = tck.StatusPass
		}
		emit(tck.AdapterMessage{Type: tck.MsgResult, ID: cmd.ID, Status: status})
	}
}

func readOneCommand(sc *bufio.Scanner) bool {
	_, ok := decodeCommand(sc)
	return ok
}

func decodeCommand(sc *bufio.Scanner) (tck.Command, bool) {
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var c tck.Command
		if err := json.Unmarshal(line, &c); err != nil {
			continue
		}
		return c, true
	}
	return tck.Command{}, false
}
