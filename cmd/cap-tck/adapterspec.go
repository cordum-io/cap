package main

import (
	"fmt"
	"strings"

	"github.com/cordum-io/cap/v2/internal/tck"
)

// parseAdapterSpec parses a "label=argv0|argv1|..." adapter flag. The label is
// everything before the first '=' (so a Windows path with a drive letter is
// fine); the argument vector is the remainder split on '|'. No shell is ever
// involved and no element is word-split, so paths and arguments may contain
// spaces.
func parseAdapterSpec(s string) (tck.AdapterSpec, error) {
	eq := strings.IndexByte(s, '=')
	if eq <= 0 {
		return tck.AdapterSpec{}, fmt.Errorf("adapter must be 'label=argv0|arg1|...', got %q", s)
	}
	label := s[:eq]
	argv := strings.Split(s[eq+1:], "|")
	if len(argv) == 0 || argv[0] == "" {
		return tck.AdapterSpec{}, fmt.Errorf("adapter %q has no executable", label)
	}
	return tck.AdapterSpec{Name: label, Argv: argv}, nil
}
