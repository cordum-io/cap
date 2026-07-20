// Command matrix-driver-go is the Go side of the CAP cross-language fixture
// matrix. It is built as an external module that consumes the published CAP Go
// SDK through a local module proxy, so a green matrix edge is evidence about
// the installed artifact rather than about repository source.
//
// Usage: matrix-driver-go produce|consume  (request JSON on stdin, response on stdout)
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fail(fmt.Errorf("usage: matrix-driver-go produce|consume"))
	}
	var (
		out any
		err error
	)
	switch os.Args[1] {
	case "produce":
		var req produceRequest
		if err = decodeStdin(&req); err == nil {
			out, err = produce(req)
		}
	case "consume":
		var req consumeRequest
		if err = decodeStdin(&req); err == nil {
			out, err = consume(req)
		}
	default:
		err = fmt.Errorf("unknown mode %q", os.Args[1])
	}
	if err != nil {
		fail(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fail(err)
	}
}

func decodeStdin(v any) error {
	if err := json.NewDecoder(os.Stdin).Decode(v); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	return nil
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "matrix-driver-go: %v\n", err)
	os.Exit(1)
}
