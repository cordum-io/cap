/// Pluggable observability callbacks for agent lifecycle events.
/// Default implementations are no-ops so users only override what they need.
pub trait MetricsHook: Send + Sync {
    fn on_job_received(&self, _job_id: &str, _topic: &str) {}
    fn on_job_completed(&self, _job_id: &str, _duration_ms: u64, _status: &str) {}
    fn on_job_failed(&self, _job_id: &str, _error_msg: &str) {}
    fn on_heartbeat_sent(&self, _worker_id: &str) {}
    fn on_progress_emitted(&self, _job_id: &str, _percent: i32) {}
    fn on_error(&self, _context: &str, _error_msg: &str) {}
}

/// No-op metrics implementation for zero overhead when metrics are not needed.
pub struct NoopMetrics;
impl MetricsHook for NoopMetrics {}
