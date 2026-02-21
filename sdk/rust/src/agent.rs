use std::collections::HashMap;
use std::sync::atomic::{AtomicI32, Ordering};
use std::sync::Arc;
use std::time::{Duration, Instant, SystemTime};

use futures::StreamExt;
use p256::ecdsa::{SigningKey, VerifyingKey};
use prost::Message;

use crate::codec::marshal_deterministic;
use crate::errors::CapError;
use crate::heartbeat::{self, HeartbeatLoop};
use crate::metrics::{MetricsHook, NoopMetrics};
use crate::middleware::{apply_middleware, Context, HandlerFn, MiddlewareFn};
use crate::pb::{bus_packet::Payload, BusPacket, JobResult, JobStatus};
use crate::signing::{sign_packet, verify_packet_signature};
use crate::subjects;

pub struct AgentConfig {
    pub nc: Option<async_nats::Client>,
    pub nats_url: String,
    pub sender_id: String,
    pub pool: String,
    pub max_parallel: i32,
    pub private_key: Option<SigningKey>,
    pub public_keys: HashMap<String, VerifyingKey>,
    pub metrics: Arc<dyn MetricsHook>,
    pub middlewares: Vec<MiddlewareFn>,
    pub heartbeat_interval: Duration,
}

pub struct Agent {
    config: AgentConfig,
    nc: async_nats::Client,
    handlers: HashMap<String, HandlerFn>,
    heartbeat_loop: Option<HeartbeatLoop>,
    active_jobs: Arc<AtomicI32>,
}

pub struct AgentBuilder {
    config: AgentConfig,
}

impl AgentBuilder {
    pub fn new(sender_id: &str) -> Self {
        AgentBuilder {
            config: AgentConfig {
                nc: None,
                nats_url: "nats://localhost:4222".into(),
                sender_id: sender_id.into(),
                pool: String::new(),
                max_parallel: 1,
                private_key: None,
                public_keys: HashMap::new(),
                metrics: Arc::new(NoopMetrics),
                middlewares: Vec::new(),
                heartbeat_interval: subjects::DEFAULT_HEARTBEAT_INTERVAL,
            },
        }
    }

    pub fn connection(mut self, nc: async_nats::Client) -> Self {
        self.config.nc = Some(nc);
        self
    }

    pub fn nats_url(mut self, url: &str) -> Self {
        self.config.nats_url = url.into();
        self
    }

    pub fn pool(mut self, pool: &str) -> Self {
        self.config.pool = pool.into();
        self
    }

    pub fn max_parallel(mut self, n: i32) -> Self {
        self.config.max_parallel = n;
        self
    }

    pub fn private_key(mut self, key: SigningKey) -> Self {
        self.config.private_key = Some(key);
        self
    }

    pub fn public_keys(mut self, keys: HashMap<String, VerifyingKey>) -> Self {
        self.config.public_keys = keys;
        self
    }

    pub fn metrics(mut self, hook: Arc<dyn MetricsHook>) -> Self {
        self.config.metrics = hook;
        self
    }

    pub fn middleware(mut self, mw: MiddlewareFn) -> Self {
        self.config.middlewares.push(mw);
        self
    }

    pub fn heartbeat_interval(mut self, interval: Duration) -> Self {
        self.config.heartbeat_interval = interval;
        self
    }

    pub async fn build(self) -> Result<Agent, CapError> {
        let nc = if let Some(nc) = self.config.nc {
            nc
        } else {
            let bus_config = crate::bus::BusConfig {
                url: self.config.nats_url.clone(),
                ..Default::default()
            };
            crate::bus::connect_nats(&bus_config).await?
        };

        Ok(Agent {
            config: self.config,
            nc,
            handlers: HashMap::new(),
            heartbeat_loop: None,
            active_jobs: Arc::new(AtomicI32::new(0)),
        })
    }
}

impl Agent {
    pub fn register(&mut self, subject: &str, handler: HandlerFn) {
        self.handlers.insert(subject.into(), handler);
    }

    pub async fn start(&mut self) -> Result<(), CapError> {
        for (subject, handler) in &self.handlers {
            let wrapped = apply_middleware(handler.clone(), &self.config.middlewares);
            let nc = self.nc.clone();
            let sender_id = self.config.sender_id.clone();
            let private_key = self.config.private_key.clone();
            let public_keys = self.config.public_keys.clone();
            let metrics = self.config.metrics.clone();
            let active_jobs = self.active_jobs.clone();

            let mut sub = nc
                .queue_subscribe(subject.clone().into(), subject.clone().into())
                .await
                .map_err(|e| CapError::subscribe_failed(&e.to_string()))?;

            tokio::spawn(async move {
                while let Some(msg) = sub.next().await {
                    let packet = match BusPacket::decode(msg.payload.as_ref()) {
                        Ok(p) => p,
                        Err(e) => {
                            metrics.on_error("parse", &format!("failed: {}", e));
                            continue;
                        }
                    };

                    if !public_keys.is_empty() {
                        if let Some(key) = public_keys.get(&packet.sender_id) {
                            if packet.signature.is_empty()
                                || verify_packet_signature(&packet, key).is_err()
                            {
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

                    active_jobs.fetch_add(1, Ordering::Relaxed);
                    let start = Instant::now();

                    let result = match wrapped(&ctx, &req) {
                        Ok(mut r) => {
                            if r.job_id.is_empty() { r.job_id = req.job_id.clone(); }
                            if r.worker_id.is_empty() { r.worker_id = sender_id.clone(); }
                            let elapsed = start.elapsed().as_millis() as u64;
                            if r.status == JobStatus::Failed as i32 {
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

                    active_jobs.fetch_sub(1, Ordering::Relaxed);

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
                            metrics.on_error("signing", "failed to sign outgoing");
                            continue;
                        }
                    }

                    let data = marshal_deterministic(&out);
                    let _ = nc.publish(subjects::SUBJECT_RESULT.into(), data.into()).await;
                }
            });
        }

        // Start heartbeat loop
        let nc_hb = self.nc.clone();
        let sender_id = self.config.sender_id.clone();
        let pool = self.config.pool.clone();
        let max_parallel = self.config.max_parallel;
        let active_jobs = self.active_jobs.clone();

        self.heartbeat_loop = Some(HeartbeatLoop::start(
            nc_hb,
            move || {
                heartbeat::heartbeat_payload(
                    &sender_id,
                    &pool,
                    active_jobs.load(Ordering::Relaxed),
                    max_parallel,
                    0.0,
                )
            },
            self.config.sender_id.clone(),
            self.config.heartbeat_interval,
            self.config.metrics.clone(),
        ));

        Ok(())
    }

    pub async fn close(&mut self) {
        if let Some(loop_) = self.heartbeat_loop.take() {
            loop_.stop().await;
        }
    }
}
