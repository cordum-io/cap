use std::sync::Arc;
use std::time::{Duration, SystemTime};

use tokio::task::JoinHandle;

use crate::codec::marshal_deterministic;
use crate::errors::CapError;
use crate::metrics::{MetricsHook, NoopMetrics};
use crate::pb::{bus_packet::Payload, BusPacket, Heartbeat};
use crate::subjects;

pub fn heartbeat_payload(
    worker_id: &str,
    pool: &str,
    active_jobs: i32,
    max_parallel: i32,
    cpu_load: f32,
) -> Vec<u8> {
    heartbeat_payload_with_progress(worker_id, pool, active_jobs, max_parallel, cpu_load, 0.0, 0, "")
}

pub fn heartbeat_payload_with_memory(
    worker_id: &str,
    pool: &str,
    active_jobs: i32,
    max_parallel: i32,
    cpu_load: f32,
    memory_load: f32,
) -> Vec<u8> {
    heartbeat_payload_with_progress(worker_id, pool, active_jobs, max_parallel, cpu_load, memory_load, 0, "")
}

pub fn heartbeat_payload_with_progress(
    worker_id: &str,
    pool: &str,
    active_jobs: i32,
    max_parallel: i32,
    cpu_load: f32,
    memory_load: f32,
    progress_pct: i32,
    last_memo: &str,
) -> Vec<u8> {
    let now = SystemTime::now()
        .duration_since(SystemTime::UNIX_EPOCH)
        .unwrap_or_default();

    let hb = Heartbeat {
        worker_id: worker_id.into(),
        pool: pool.into(),
        active_jobs,
        max_parallel_jobs: max_parallel,
        cpu_load,
        memory_load,
        progress_pct,
        last_memo: last_memo.into(),
        ..Default::default()
    };

    let packet = BusPacket {
        sender_id: worker_id.into(),
        protocol_version: subjects::DEFAULT_PROTOCOL_VERSION,
        created_at: Some(prost_types::Timestamp {
            seconds: now.as_secs() as i64,
            nanos: now.subsec_nanos() as i32,
        }),
        payload: Some(Payload::Heartbeat(hb)),
        ..Default::default()
    };

    marshal_deterministic(&packet)
}

pub async fn emit_heartbeat(nc: &async_nats::Client, payload: &[u8]) -> Result<(), CapError> {
    nc.publish(subjects::SUBJECT_HEARTBEAT.into(), payload.to_vec().into())
        .await
        .map_err(|e| CapError::publish_failed(&e.to_string()))
}

/// Periodically emits heartbeats using a tokio task.
/// Call `stop()` to cancel the loop.
pub struct HeartbeatLoop {
    cancel_tx: tokio::sync::watch::Sender<bool>,
    handle: JoinHandle<()>,
}

impl HeartbeatLoop {
    pub fn start<F>(
        nc: async_nats::Client,
        payload_fn: F,
        worker_id: String,
        interval: Duration,
        metrics: Arc<dyn MetricsHook>,
    ) -> Self
    where
        F: Fn() -> Vec<u8> + Send + Sync + 'static,
    {
        let (cancel_tx, mut cancel_rx) = tokio::sync::watch::channel(false);

        let handle = tokio::spawn(async move {
            let mut ticker = tokio::time::interval(interval);
            loop {
                tokio::select! {
                    _ = ticker.tick() => {
                        let payload = payload_fn();
                        if emit_heartbeat(&nc, &payload).await.is_ok() {
                            metrics.on_heartbeat_sent(&worker_id);
                        }
                    }
                    _ = cancel_rx.changed() => {
                        break;
                    }
                }
            }
        });

        HeartbeatLoop { cancel_tx, handle }
    }

    pub async fn stop(self) {
        let _ = self.cancel_tx.send(true);
        let _ = self.handle.await;
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use prost::Message;

    #[test]
    fn heartbeat_payload_valid() {
        let bytes = heartbeat_payload("w-1", "pool-a", 3, 8, 42.5);
        assert!(!bytes.is_empty());

        let packet = BusPacket::decode(bytes.as_slice()).unwrap();
        assert_eq!(packet.sender_id, "w-1");
        assert_eq!(packet.protocol_version, 1);

        if let Some(Payload::Heartbeat(hb)) = packet.payload {
            assert_eq!(hb.worker_id, "w-1");
            assert_eq!(hb.pool, "pool-a");
            assert_eq!(hb.active_jobs, 3);
            assert_eq!(hb.max_parallel_jobs, 8);
            assert!((hb.cpu_load - 42.5).abs() < 0.01);
        } else {
            panic!("expected heartbeat payload");
        }
    }

    #[test]
    fn with_memory_sets_fields() {
        let bytes = heartbeat_payload_with_memory("w-1", "p", 1, 4, 10.0, 55.0);
        let packet = BusPacket::decode(bytes.as_slice()).unwrap();
        if let Some(Payload::Heartbeat(hb)) = packet.payload {
            assert!((hb.memory_load - 55.0).abs() < 0.01);
            assert_eq!(hb.progress_pct, 0);
        } else {
            panic!("expected heartbeat payload");
        }
    }

    #[test]
    fn with_progress_sets_all_fields() {
        let bytes = heartbeat_payload_with_progress("w-2", "gpu", 5, 10, 80.0, 60.0, 75, "step-3");
        let packet = BusPacket::decode(bytes.as_slice()).unwrap();
        if let Some(Payload::Heartbeat(hb)) = packet.payload {
            assert_eq!(hb.progress_pct, 75);
            assert_eq!(hb.last_memo, "step-3");
        } else {
            panic!("expected heartbeat payload");
        }
    }
}
