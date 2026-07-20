use std::time::SystemTime;

use crate::codec::marshal_deterministic;
use crate::errors::CapError;
use crate::pb::{bus_packet::Payload, BusPacket, JobCancel, JobProgress};
use crate::subjects;

pub fn progress_payload(
    sender_id: &str,
    job_id: &str,
    step_id: &str,
    percent: i32,
    message: &str,
) -> Vec<u8> {
    let now = SystemTime::now()
        .duration_since(SystemTime::UNIX_EPOCH)
        .unwrap_or_default();

    let packet = BusPacket {
        sender_id: sender_id.into(),
        protocol_version: subjects::DEFAULT_PROTOCOL_VERSION,
        created_at: Some(prost_types::Timestamp {
            seconds: now.as_secs() as i64,
            nanos: now.subsec_nanos() as i32,
        }),
        payload: Some(Payload::JobProgress(JobProgress {
            job_id: job_id.into(),
            step_id: step_id.into(),
            percent,
            message: message.into(),
            ..Default::default()
        })),
        ..Default::default()
    };

    marshal_deterministic(&packet)
}

pub fn cancel_payload(
    sender_id: &str,
    job_id: &str,
    reason: &str,
    requested_by: &str,
) -> Vec<u8> {
    let now = SystemTime::now()
        .duration_since(SystemTime::UNIX_EPOCH)
        .unwrap_or_default();

    let packet = BusPacket {
        sender_id: sender_id.into(),
        protocol_version: subjects::DEFAULT_PROTOCOL_VERSION,
        created_at: Some(prost_types::Timestamp {
            seconds: now.as_secs() as i64,
            nanos: now.subsec_nanos() as i32,
        }),
        payload: Some(Payload::JobCancel(JobCancel {
            job_id: job_id.into(),
            reason: reason.into(),
            requested_by: requested_by.into(),
        })),
        ..Default::default()
    };

    marshal_deterministic(&packet)
}

pub async fn emit_progress(nc: &async_nats::Client, payload: &[u8]) -> Result<(), CapError> {
    nc.publish(subjects::SUBJECT_PROGRESS, payload.to_vec().into())
        .await
        .map_err(|e| CapError::publish_failed(&e.to_string()))
}

pub async fn emit_cancel(nc: &async_nats::Client, payload: &[u8]) -> Result<(), CapError> {
    nc.publish(subjects::SUBJECT_CANCEL, payload.to_vec().into())
        .await
        .map_err(|e| CapError::publish_failed(&e.to_string()))
}

#[cfg(test)]
mod tests {
    use super::*;
    use prost::Message;

    #[test]
    fn progress_payload_valid() {
        let bytes = progress_payload("s-1", "j-42", "step-3", 75, "processing");
        assert!(!bytes.is_empty());

        let packet = BusPacket::decode(bytes.as_slice()).unwrap();
        assert_eq!(packet.sender_id, "s-1");

        if let Some(Payload::JobProgress(p)) = packet.payload {
            assert_eq!(p.job_id, "j-42");
            assert_eq!(p.step_id, "step-3");
            assert_eq!(p.percent, 75);
            assert_eq!(p.message, "processing");
        } else {
            panic!("expected job_progress payload");
        }
    }

    #[test]
    fn cancel_payload_valid() {
        let bytes = cancel_payload("s-1", "j-99", "timeout", "admin");
        assert!(!bytes.is_empty());

        let packet = BusPacket::decode(bytes.as_slice()).unwrap();
        if let Some(Payload::JobCancel(c)) = packet.payload {
            assert_eq!(c.job_id, "j-99");
            assert_eq!(c.reason, "timeout");
            assert_eq!(c.requested_by, "admin");
        } else {
            panic!("expected job_cancel payload");
        }
    }

    #[test]
    fn progress_zero_percent() {
        let bytes = progress_payload("s", "j", "st", 0, "");
        let packet = BusPacket::decode(bytes.as_slice()).unwrap();
        if let Some(Payload::JobProgress(p)) = packet.payload {
            assert_eq!(p.percent, 0);
        } else {
            panic!("expected job_progress payload");
        }
    }
}
