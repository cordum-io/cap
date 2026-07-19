package governance

import (
	"testing"
	"time"
)

func TestOverdueTriage(t *testing.T) {
	now := asOf("2026-07-19")
	items := []TriageItem{
		{Number: 1, CreatedAt: asOf("2026-07-01"), Labels: []string{"needs-triage"}},        // 18d old, overdue
		{Number: 2, CreatedAt: asOf("2026-07-17"), Labels: []string{"needs-triage"}},        // 2d old, ok
		{Number: 3, CreatedAt: asOf("2026-06-01"), Labels: []string{"bug"}},                 // triaged already
		{Number: 4, CreatedAt: asOf("2026-07-01"), Labels: []string{"needs-triage", "bug"}}, // overdue
	}
	over := OverdueTriage(items, now, 7)
	if len(over) != 2 {
		t.Fatalf("want 2 overdue, got %d (%+v)", len(over), over)
	}
	if over[0].Number != 1 || over[1].Number != 4 {
		t.Fatalf("unexpected overdue set: %+v", over)
	}
}

func TestExactlyAtSLAIsNotOverdue(t *testing.T) {
	now := asOf("2026-07-08")
	items := []TriageItem{{Number: 1, CreatedAt: asOf("2026-07-01"), Labels: []string{"needs-triage"}}}
	if o := OverdueTriage(items, now, 7); len(o) != 0 {
		t.Fatalf("an item exactly at the 7-day SLA is not yet overdue, got %+v", o)
	}
}

func TestOverdueIgnoresTimezoneNoise(t *testing.T) {
	now := time.Date(2026, 7, 19, 3, 0, 0, 0, time.UTC)
	items := []TriageItem{{Number: 1, CreatedAt: time.Date(2026, 7, 1, 23, 0, 0, 0, time.UTC), Labels: []string{"needs-triage"}}}
	if o := OverdueTriage(items, now, 7); len(o) != 1 {
		t.Fatalf("want 1 overdue across a timezone-ish gap, got %+v", o)
	}
}
