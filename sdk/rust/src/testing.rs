use std::sync::{Arc, Mutex};

use crate::errors::CapError;
use crate::metrics::MetricsHook;
use crate::middleware::{Context, HandlerFn};
use crate::pb::{JobRequest, JobResult};

/// Tracks published messages for test assertions.
#[derive(Clone, Default)]
pub struct MockBus {
    pub published: Arc<Mutex<Vec<(String, Vec<u8>)>>>,
}

impl MockBus {
    pub fn new() -> Self {
        MockBus::default()
    }

    pub fn publish(&self, subject: &str, data: Vec<u8>) {
        self.published.lock().unwrap().push((subject.into(), data));
    }

    pub fn published_count(&self) -> usize {
        self.published.lock().unwrap().len()
    }

    pub fn published_to(&self, subject: &str) -> Vec<Vec<u8>> {
        self.published
            .lock()
            .unwrap()
            .iter()
            .filter(|(s, _)| s == subject)
            .map(|(_, d)| d.clone())
            .collect()
    }

    pub fn clear(&self) {
        self.published.lock().unwrap().clear();
    }
}

/// Submit a job request to a handler and return the result.
/// Useful for quick unit tests without NATS infrastructure.
pub fn submit_and_wait(handler: &HandlerFn, req: &JobRequest) -> Result<JobResult, CapError> {
    let ctx = Context {
        trace_id: "test-trace".into(),
        sender_id: "test-sender".into(),
        packet: None,
    };
    handler(&ctx, req)
}

/// Recording implementation of MetricsHook for test assertions.
#[derive(Default)]
pub struct RecordingMetrics {
    pub received_jobs: Mutex<Vec<String>>,
    pub completed_jobs: Mutex<Vec<String>>,
    pub failed_jobs: Mutex<Vec<String>>,
    pub heartbeats: Mutex<Vec<String>>,
    pub errors: Mutex<Vec<String>>,
}

impl MetricsHook for RecordingMetrics {
    fn on_job_received(&self, job_id: &str, _topic: &str) {
        self.received_jobs.lock().unwrap().push(job_id.into());
    }
    fn on_job_completed(&self, job_id: &str, _duration_ms: u64, _status: &str) {
        self.completed_jobs.lock().unwrap().push(job_id.into());
    }
    fn on_job_failed(&self, job_id: &str, _error_msg: &str) {
        self.failed_jobs.lock().unwrap().push(job_id.into());
    }
    fn on_heartbeat_sent(&self, worker_id: &str) {
        self.heartbeats.lock().unwrap().push(worker_id.into());
    }
    fn on_error(&self, context: &str, error_msg: &str) {
        self.errors.lock().unwrap().push(format!("{}: {}", context, error_msg));
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Arc;

    #[test]
    fn submit_and_wait_basic() {
        let handler: HandlerFn = Arc::new(|_ctx, req: &JobRequest| {
            Ok(JobResult {
                job_id: req.job_id.clone(),
                status: 5, // SUCCEEDED
                ..Default::default()
            })
        });

        let req = JobRequest {
            job_id: "j-1".into(),
            topic: "test".into(),
            ..Default::default()
        };

        let result = submit_and_wait(&handler, &req).unwrap();
        assert_eq!(result.job_id, "j-1");
        assert_eq!(result.status, 5);
    }

    #[test]
    fn mock_bus_tracks_publishes() {
        let bus = MockBus::new();
        bus.publish("sys.heartbeat", vec![1, 2, 3]);
        bus.publish("sys.job.result", vec![4, 5]);

        assert_eq!(bus.published_count(), 2);
        assert_eq!(bus.published_to("sys.heartbeat").len(), 1);
        assert_eq!(bus.published_to("sys.job.result").len(), 1);
        assert_eq!(bus.published_to("sys.job.submit").len(), 0);
    }

    #[test]
    fn recording_metrics_tracks_events() {
        let m = RecordingMetrics::default();
        m.on_job_received("j-1", "test");
        m.on_job_completed("j-1", 100, "SUCCEEDED");
        m.on_heartbeat_sent("w-1");

        assert_eq!(m.received_jobs.lock().unwrap().len(), 1);
        assert_eq!(m.completed_jobs.lock().unwrap().len(), 1);
        assert_eq!(m.heartbeats.lock().unwrap().len(), 1);
    }
}
