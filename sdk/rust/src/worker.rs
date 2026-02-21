use std::collections::HashMap;
use std::sync::Arc;
use std::time::{Instant, SystemTime};

use p256::ecdsa::{SigningKey, VerifyingKey};
use prost::Message;

use crate::codec::marshal_deterministic;
use crate::errors::CapError;
use crate::metrics::{MetricsHook, NoopMetrics};
use crate::middleware::{apply_middleware, Context, HandlerFn, MiddlewareFn};
use crate::pb::{bus_packet::Payload, BusPacket, JobRequest, JobResult, JobStatus};
use crate::signing::{sign_packet, verify_packet_signature};
use crate::subjects;

pub struct Worker {
    nc: async_nats::Client,
    subject: String,
    sender_id: String,
    handler: HandlerFn,
    middlewares: Vec<MiddlewareFn>,
    private_key: Option<SigningKey>,
    public_keys: HashMap<String, VerifyingKey>,
    metrics: Arc<dyn MetricsHook>,
}

impl Worker {
    pub fn new(
        nc: async_nats::Client,
        subject: String,
        sender_id: String,
        handler: HandlerFn,
    ) -> Self {
        Worker {
            nc,
            subject,
            sender_id,
            handler,
            middlewares: Vec::new(),
            private_key: None,
            public_keys: HashMap::new(),
            metrics: Arc::new(NoopMetrics),
        }
    }

    pub fn with_private_key(mut self, key: SigningKey) -> Self {
        self.private_key = Some(key);
        self
    }

    pub fn with_public_keys(mut self, keys: HashMap<String, VerifyingKey>) -> Self {
        self.public_keys = keys;
        self
    }

    pub fn with_metrics(mut self, metrics: Arc<dyn MetricsHook>) -> Self {
        self.metrics = metrics;
        self
    }

    pub fn use_middleware(&mut self, mw: MiddlewareFn) {
        self.middlewares.push(mw);
    }

    pub async fn start(&self) -> Result<(), CapError> {
        let wrapped = apply_middleware(self.handler.clone(), &self.middlewares);

        let mut sub = self
            .nc
            .queue_subscribe(self.subject.clone().into(), self.subject.clone().into())
            .await
            .map_err(|e| CapError::subscribe_failed(&e.to_string()))?;

        let nc = self.nc.clone();
        let sender_id = self.sender_id.clone();
        let private_key = self.private_key.clone();
        let public_keys = self.public_keys.clone();
        let metrics = self.metrics.clone();

        tokio::spawn(async move {
            while let Some(msg) = sub.next().await {
                let packet = match BusPacket::decode(msg.payload.as_ref()) {
                    Ok(p) => p,
                    Err(e) => {
                        metrics.on_error("parse", &format!("failed to parse: {}", e));
                        continue;
                    }
                };

                if !public_keys.is_empty() {
                    if let Some(key) = public_keys.get(&packet.sender_id) {
                        if packet.signature.is_empty() {
                            continue;
                        }
                        if verify_packet_signature(&packet, key).is_err() {
                            continue;
                        }
                    } else {
                        continue;
                    }
                }

                let req = match &packet.payload {
                    Some(Payload::JobRequest(r)) => r.clone(),
                    _ => continue,
                };

                metrics.on_job_received(&req.job_id, &req.topic);

                let ctx = Context {
                    trace_id: packet.trace_id.clone(),
                    sender_id: packet.sender_id.clone(),
                    packet: Some(packet.clone()),
                };

                let start = Instant::now();
                let result = match wrapped(&ctx, &req) {
                    Ok(mut r) => {
                        if r.job_id.is_empty() {
                            r.job_id = req.job_id.clone();
                        }
                        if r.worker_id.is_empty() {
                            r.worker_id = sender_id.clone();
                        }
                        let elapsed = start.elapsed().as_millis() as u64;
                        if r.status == JobStatus::Failed as i32 {
                            // FAILED
                            metrics.on_job_failed(&r.job_id, &r.error_message);
                        } else {
                            metrics.on_job_completed(&r.job_id, elapsed, "SUCCEEDED");
                        }
                        r
                    }
                    Err(e) => {
                        metrics.on_job_failed(&req.job_id, &e.to_string());
                        JobResult {
                            job_id: req.job_id.clone(),
                            worker_id: sender_id.clone(),
                            status: JobStatus::Failed as i32,
                            error_message: e.to_string(),
                            ..Default::default()
                        }
                    }
                };

                let now = SystemTime::now()
                    .duration_since(SystemTime::UNIX_EPOCH)
                    .unwrap_or_default();

                let mut out = BusPacket {
                    trace_id: packet.trace_id.clone(),
                    sender_id: sender_id.clone(),
                    protocol_version: subjects::DEFAULT_PROTOCOL_VERSION,
                    created_at: Some(prost_types::Timestamp {
                        seconds: now.as_secs() as i64,
                        nanos: now.subsec_nanos() as i32,
                    }),
                    payload: Some(Payload::JobResult(result)),
                    ..Default::default()
                };

                if let Some(ref key) = private_key {
                    if sign_packet(&mut out, key).is_err() {
                        metrics.on_error("signing", "failed to sign outgoing packet");
                        continue;
                    }
                }

                let data = marshal_deterministic(&out);
                let _ = nc
                    .publish(subjects::SUBJECT_RESULT.into(), data.into())
                    .await;
            }
        });

        Ok(())
    }
}

use futures::StreamExt;
