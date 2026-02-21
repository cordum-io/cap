use cap_sdk::codec::{marshal_deterministic, marshal_unsigned_for_signature};
use cap_sdk::pb::{bus_packet::Payload, BusPacket, Heartbeat, JobRequest, JobResult};
use prost::Message;

fn make_job_request_packet() -> BusPacket {
    BusPacket {
        trace_id: "trace-codec".into(),
        sender_id: "sender-codec".into(),
        protocol_version: 1,
        created_at: Some(prost_types::Timestamp {
            seconds: 1704164645,
            nanos: 0,
        }),
        payload: Some(Payload::JobRequest(JobRequest {
            job_id: "job-codec-1".into(),
            topic: "codec.test".into(),
            ..Default::default()
        })),
        ..Default::default()
    }
}

fn make_heartbeat_packet() -> BusPacket {
    BusPacket {
        trace_id: "trace-hb".into(),
        sender_id: "worker-hb".into(),
        protocol_version: 1,
        created_at: Some(prost_types::Timestamp {
            seconds: 1704164645,
            nanos: 0,
        }),
        payload: Some(Payload::Heartbeat(Heartbeat {
            worker_id: "worker-hb".into(),
            pool: "default".into(),
            active_jobs: 2,
            max_parallel_jobs: 8,
            cpu_load: 45.5,
            ..Default::default()
        })),
        ..Default::default()
    }
}

#[test]
fn deterministic_100_iterations() {
    let packet = make_job_request_packet();
    let first = marshal_deterministic(&packet);
    assert!(!first.is_empty());

    for i in 0..100 {
        let next = marshal_deterministic(&packet);
        assert_eq!(first, next, "iteration {} produced different bytes", i);
    }
}

#[test]
fn deterministic_heartbeat_100_iterations() {
    let packet = make_heartbeat_packet();
    let first = marshal_deterministic(&packet);

    for i in 0..100 {
        assert_eq!(
            first,
            marshal_deterministic(&packet),
            "heartbeat iteration {} differs",
            i
        );
    }
}

#[test]
fn round_trip_job_request() {
    let packet = make_job_request_packet();
    let bytes = marshal_deterministic(&packet);
    let decoded = BusPacket::decode(bytes.as_slice()).unwrap();

    assert_eq!(decoded.trace_id, "trace-codec");
    assert_eq!(decoded.sender_id, "sender-codec");
    assert_eq!(decoded.protocol_version, 1);

    if let Some(Payload::JobRequest(req)) = decoded.payload {
        assert_eq!(req.job_id, "job-codec-1");
        assert_eq!(req.topic, "codec.test");
    } else {
        panic!("expected JobRequest payload");
    }
}

#[test]
fn round_trip_heartbeat() {
    let packet = make_heartbeat_packet();
    let bytes = marshal_deterministic(&packet);
    let decoded = BusPacket::decode(bytes.as_slice()).unwrap();

    if let Some(Payload::Heartbeat(hb)) = decoded.payload {
        assert_eq!(hb.worker_id, "worker-hb");
        assert_eq!(hb.pool, "default");
        assert_eq!(hb.active_jobs, 2);
        assert_eq!(hb.max_parallel_jobs, 8);
        assert!((hb.cpu_load - 45.5).abs() < 0.01);
    } else {
        panic!("expected Heartbeat payload");
    }
}

#[test]
fn round_trip_job_result() {
    let packet = BusPacket {
        trace_id: "trace-res".into(),
        sender_id: "worker-1".into(),
        protocol_version: 1,
        created_at: Some(prost_types::Timestamp {
            seconds: 1704164645,
            nanos: 0,
        }),
        payload: Some(Payload::JobResult(JobResult {
            job_id: "job-res-1".into(),
            worker_id: "worker-1".into(),
            status: 5, // SUCCEEDED
            execution_ms: 150,
            ..Default::default()
        })),
        ..Default::default()
    };

    let bytes = marshal_deterministic(&packet);
    let decoded = BusPacket::decode(bytes.as_slice()).unwrap();

    if let Some(Payload::JobResult(res)) = decoded.payload {
        assert_eq!(res.job_id, "job-res-1");
        assert_eq!(res.worker_id, "worker-1");
        assert_eq!(res.status, 5);
        assert_eq!(res.execution_ms, 150);
    } else {
        panic!("expected JobResult payload");
    }
}

#[test]
fn unsigned_for_signature_strips_signature() {
    let mut packet = make_job_request_packet();
    packet.signature = b"some-fake-signature-data".to_vec();

    let unsigned_bytes = marshal_unsigned_for_signature(&packet);
    let decoded = BusPacket::decode(unsigned_bytes.as_slice()).unwrap();

    assert!(decoded.signature.is_empty());
    assert_eq!(decoded.trace_id, "trace-codec");
    assert_eq!(decoded.sender_id, "sender-codec");
}

#[test]
fn unsigned_for_signature_matches_clean_packet() {
    let mut packet = make_job_request_packet();
    packet.signature = b"to-be-stripped".to_vec();

    let unsigned_bytes = marshal_unsigned_for_signature(&packet);

    let mut clean = packet.clone();
    clean.signature = Vec::new();
    let clean_bytes = marshal_deterministic(&clean);

    assert_eq!(unsigned_bytes, clean_bytes);
}

#[test]
fn empty_packet_serializes() {
    let empty = BusPacket::default();
    let bytes = marshal_deterministic(&empty);
    // Empty proto3 message with all default fields encodes to empty bytes
    assert!(bytes.is_empty());

    // But we can still decode it
    let decoded = BusPacket::decode(bytes.as_slice()).unwrap();
    assert_eq!(decoded, empty);
}

#[test]
fn different_packets_produce_different_bytes() {
    let pkt1 = make_job_request_packet();
    let pkt2 = make_heartbeat_packet();

    let bytes1 = marshal_deterministic(&pkt1);
    let bytes2 = marshal_deterministic(&pkt2);

    assert_ne!(bytes1, bytes2);
}
