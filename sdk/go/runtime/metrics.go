package runtime

import capsdk "github.com/cordum-io/cap/v2/sdk/go"

// MetricsHook receives lifecycle events for observability.
// See capsdk.MetricsHook for the full interface definition.
type MetricsHook = capsdk.MetricsHook

// NoopMetrics is a no-op implementation with zero overhead.
var NoopMetrics MetricsHook = capsdk.NoopMetrics
