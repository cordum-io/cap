use std::collections::HashMap;
use std::time::SystemTime;

use p256::ecdsa::SigningKey;
use prost::Message;

use crate::codec::marshal_deterministic;
use crate::errors::CapError;
use crate::pb::{bus_packet::Payload, BusPacket, JobRequest};
use crate::signing::sign_packet;
use crate::subjects;

pub struct Client {
    nc: async_nats::Client,
    sender_id: String,
    private_key: Option<SigningKey>,
}

impl Client {
    pub fn new(nc: async_nats::Client, sender_id: String) -> Self {
        Client {
            nc,
            sender_id,
            private_key: None,
        }
    }

    pub fn with_private_key(mut self, key: SigningKey) -> Self {
        self.private_key = Some(key);
        self
    }

    pub async fn submit(
        &self,
        trace_id: &str,
        req: &JobRequest,
    ) -> Result<(), CapError> {
        self.submit_with_version(trace_id, req, subjects::DEFAULT_PROTOCOL_VERSION)
            .await
    }

    pub async fn submit_with_version(
        &self,
        trace_id: &str,
        req: &JobRequest,
        protocol_version: i32,
    ) -> Result<(), CapError> {
        let now = SystemTime::now()
            .duration_since(SystemTime::UNIX_EPOCH)
            .unwrap_or_default();

        let mut packet = BusPacket {
            trace_id: trace_id.into(),
            sender_id: self.sender_id.clone(),
            protocol_version,
            created_at: Some(prost_types::Timestamp {
                seconds: now.as_secs() as i64,
                nanos: now.subsec_nanos() as i32,
            }),
            payload: Some(Payload::JobRequest(req.clone())),
            ..Default::default()
        };

        if let Some(ref key) = self.private_key {
            sign_packet(&mut packet, key)?;
        }

        let data = marshal_deterministic(&packet);
        self.nc
            .publish(subjects::SUBJECT_SUBMIT.into(), data.into())
            .await
            .map_err(|e| CapError::publish_failed(&e.to_string()))?;

        Ok(())
    }
}
