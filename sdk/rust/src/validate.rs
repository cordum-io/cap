use crate::errors::CapError;
use crate::pb::{BusPacket, JobRequest, JobResult};

pub fn validate_job_request(req: &JobRequest) -> Result<(), CapError> {
    if req.job_id.is_empty() {
        return Err(CapError::validation("job_id", "must not be empty"));
    }
    if req.topic.is_empty() {
        return Err(CapError::validation("topic", "must not be empty"));
    }
    if req.step_index < 0 {
        return Err(CapError::validation("step_index", "must be >= 0"));
    }
    if let Some(ref budget) = req.budget {
        if budget.max_input_tokens < 0 {
            return Err(CapError::validation("budget.max_input_tokens", "must be >= 0"));
        }
        if budget.max_output_tokens < 0 {
            return Err(CapError::validation("budget.max_output_tokens", "must be >= 0"));
        }
        if budget.max_total_tokens < 0 {
            return Err(CapError::validation("budget.max_total_tokens", "must be >= 0"));
        }
        if budget.deadline_ms < 0 {
            return Err(CapError::validation("budget.deadline_ms", "must be >= 0"));
        }
    }
    Ok(())
}

pub fn validate_job_result(res: &JobResult) -> Result<(), CapError> {
    if res.job_id.is_empty() {
        return Err(CapError::validation("job_id", "must not be empty"));
    }
    if res.status == 0 {
        return Err(CapError::validation("status", "must not be UNSPECIFIED"));
    }
    if res.worker_id.is_empty() {
        return Err(CapError::validation("worker_id", "must not be empty"));
    }
    if res.execution_ms < 0 {
        return Err(CapError::validation("execution_ms", "must be >= 0"));
    }
    Ok(())
}

pub fn validate_bus_packet(pkt: &BusPacket) -> Result<(), CapError> {
    if pkt.trace_id.is_empty() {
        return Err(CapError::validation("trace_id", "must not be empty"));
    }
    if pkt.sender_id.is_empty() {
        return Err(CapError::validation("sender_id", "must not be empty"));
    }
    if pkt.protocol_version <= 0 {
        return Err(CapError::validation("protocol_version", "must be > 0"));
    }
    if pkt.created_at.is_none() {
        return Err(CapError::validation("created_at", "must not be null"));
    }
    if pkt.payload.is_none() {
        return Err(CapError::validation("payload", "must not be empty"));
    }
    match &pkt.payload {
        Some(crate::pb::bus_packet::Payload::JobRequest(req)) => validate_job_request(req)?,
        Some(crate::pb::bus_packet::Payload::JobResult(res)) => validate_job_result(res)?,
        _ => {}
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn valid_job_request() {
        let req = JobRequest {
            job_id: "j-1".into(),
            topic: "test".into(),
            ..Default::default()
        };
        assert!(validate_job_request(&req).is_ok());
    }

    #[test]
    fn empty_job_id_rejected() {
        let req = JobRequest {
            topic: "test".into(),
            ..Default::default()
        };
        assert!(validate_job_request(&req).is_err());
    }

    #[test]
    fn empty_topic_rejected() {
        let req = JobRequest {
            job_id: "j".into(),
            ..Default::default()
        };
        assert!(validate_job_request(&req).is_err());
    }

    #[test]
    fn valid_bus_packet() {
        use crate::pb::bus_packet::Payload;
        let pkt = BusPacket {
            trace_id: "t".into(),
            sender_id: "s".into(),
            protocol_version: 1,
            created_at: Some(prost_types::Timestamp { seconds: 1, nanos: 0 }),
            payload: Some(Payload::JobRequest(JobRequest {
                job_id: "j".into(),
                topic: "t".into(),
                ..Default::default()
            })),
            ..Default::default()
        };
        assert!(validate_bus_packet(&pkt).is_ok());
    }

    #[test]
    fn empty_sender_rejected() {
        let pkt = BusPacket {
            trace_id: "t".into(),
            protocol_version: 1,
            ..Default::default()
        };
        assert!(validate_bus_packet(&pkt).is_err());
    }
}
