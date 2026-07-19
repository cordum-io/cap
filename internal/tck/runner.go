package tck

import (
	"context"
	"fmt"
	"time"
)

const defaultCaseDeadline = 30 * time.Second

// Runner executes a scenario suite against one adapter and normalizes the
// outcomes into a Report. The clock is injectable so durations are
// deterministic under test.
type Runner struct {
	CaseDeadline time.Duration
	ToolVersion  string
	now          func() time.Time
}

// NewRunner returns a runner with production defaults.
func NewRunner() *Runner {
	return &Runner{
		CaseDeadline: defaultCaseDeadline,
		ToolVersion:  HarnessVersion,
		now:          time.Now,
	}
}

// Run executes every scenario in the profile against the adapter and returns a
// report. A scenario the adapter's role does not own is recorded as a
// non-applicable N/A and never sent. Cancelling ctx aborts the remaining cases.
func (r *Runner) Run(ctx context.Context, a *Adapter, suite *Suite, profile string) Report {
	role := a.Handshake().Role
	scenarios := suite.Select(profile)
	cases := make([]CaseResult, 0, len(scenarios))
	for _, sc := range scenarios {
		cases = append(cases, r.runCase(ctx, a, sc, role))
	}
	return NewReport(a.Name(), a.Handshake(), profile, r.ToolVersion, cases)
}

// runCase runs one scenario, mapping role mismatch to N/A and any driver error
// to ERROR so a fault is never silently dropped.
func (r *Runner) runCase(ctx context.Context, a *Adapter, sc Scenario, role Role) CaseResult {
	cr := CaseResult{ID: sc.ID, Profile: sc.Profile, Role: sc.Role, Required: sc.Required}
	if sc.Role != role {
		cr.Applicable = false
		cr.Status = StatusNA
		cr.Detail = fmt.Sprintf("adapter role %q does not own scenario role %q", role, sc.Role)
		return cr
	}
	cr.Applicable = true
	start := r.clock()
	msg, err := a.Run(ctx, Command{Type: MsgRun, ID: sc.ID, Scenario: sc.ID, Params: sc.Params}, r.CaseDeadline)
	cr.DurationMs = r.clock().Sub(start).Milliseconds()
	if err != nil {
		cr.Status = StatusError
		cr.Detail = err.Error()
		return cr
	}
	cr.Status = msg.Status
	cr.Detail = msg.Detail
	cr.Evidence = msg.Evidence
	return cr
}

func (r *Runner) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}
