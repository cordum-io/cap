use cap_sdk::codec::marshal_deterministic;
use cap_sdk::pb::{bus_packet::Payload, BusPacket, Heartbeat, JobRequest, JobResult};
use cap_sdk::signing::{sign_packet, verify_packet_signature};
use p256::ecdsa::{SigningKey, VerifyingKey};
use rand_core::OsRng;

fn make_packet() -> BusPacket {
    BusPacket {
        trace_id: "trace-sign".into(),
        sender_id: "sender-sign".into(),
        protocol_version: 1,
        created_at: Some(prost_types::Timestamp {
            seconds: 1704164645,
            nanos: 0,
        }),
        payload: Some(Payload::JobRequest(JobRequest {
            job_id: "job-sign-1".into(),
            topic: "sign.test".into(),
            ..Default::default()
        })),
        ..Default::default()
    }
}

fn make_key_pair() -> (SigningKey, VerifyingKey) {
    let sk = SigningKey::random(&mut OsRng);
    let vk = VerifyingKey::from(&sk);
    (sk, vk)
}

#[test]
fn sign_and_verify_job_request() {
    let (sk, vk) = make_key_pair();
    let mut packet = make_packet();

    sign_packet(&mut packet, &sk).unwrap();
    assert!(!packet.signature.is_empty());

    verify_packet_signature(&packet, &vk).unwrap();
}

#[test]
fn sign_and_verify_heartbeat() {
    let (sk, vk) = make_key_pair();
    let mut packet = BusPacket {
        trace_id: "trace-hb".into(),
        sender_id: "worker-hb".into(),
        protocol_version: 1,
        payload: Some(Payload::Heartbeat(Heartbeat {
            worker_id: "worker-hb".into(),
            pool: "gpu".into(),
            active_jobs: 3,
            max_parallel_jobs: 10,
            cpu_load: 75.0,
            ..Default::default()
        })),
        ..Default::default()
    };

    sign_packet(&mut packet, &sk).unwrap();
    verify_packet_signature(&packet, &vk).unwrap();
}

#[test]
fn sign_and_verify_job_result() {
    let (sk, vk) = make_key_pair();
    let mut packet = BusPacket {
        trace_id: "trace-res".into(),
        sender_id: "worker-1".into(),
        protocol_version: 1,
        payload: Some(Payload::JobResult(JobResult {
            job_id: "job-res-1".into(),
            worker_id: "worker-1".into(),
            status: 5,
            ..Default::default()
        })),
        ..Default::default()
    };

    sign_packet(&mut packet, &sk).unwrap();
    verify_packet_signature(&packet, &vk).unwrap();
}

#[test]
fn tamper_trace_id_detected() {
    let (sk, vk) = make_key_pair();
    let mut packet = make_packet();
    sign_packet(&mut packet, &sk).unwrap();

    packet.trace_id = "tampered-trace".into();
    assert!(verify_packet_signature(&packet, &vk).is_err());
}

#[test]
fn tamper_sender_id_detected() {
    let (sk, vk) = make_key_pair();
    let mut packet = make_packet();
    sign_packet(&mut packet, &sk).unwrap();

    packet.sender_id = "tampered-sender".into();
    assert!(verify_packet_signature(&packet, &vk).is_err());
}

#[test]
fn tamper_payload_detected() {
    let (sk, vk) = make_key_pair();
    let mut packet = make_packet();
    sign_packet(&mut packet, &sk).unwrap();

    packet.payload = Some(Payload::JobRequest(JobRequest {
        job_id: "tampered-job".into(),
        topic: "tampered.topic".into(),
        ..Default::default()
    }));
    assert!(verify_packet_signature(&packet, &vk).is_err());
}

#[test]
fn tamper_protocol_version_detected() {
    let (sk, vk) = make_key_pair();
    let mut packet = make_packet();
    sign_packet(&mut packet, &sk).unwrap();

    packet.protocol_version = 99;
    assert!(verify_packet_signature(&packet, &vk).is_err());
}

#[test]
fn empty_signature_rejected() {
    let (_, vk) = make_key_pair();
    let packet = make_packet();
    assert!(verify_packet_signature(&packet, &vk).is_err());
}

#[test]
fn garbage_signature_rejected() {
    let (_, vk) = make_key_pair();
    let mut packet = make_packet();
    packet.signature = vec![0xFF, 0xFE, 0xFD, 0xFC];
    assert!(verify_packet_signature(&packet, &vk).is_err());
}

#[test]
fn wrong_key_rejected() {
    let (sk1, _) = make_key_pair();
    let (_, vk2) = make_key_pair();

    let mut packet = make_packet();
    sign_packet(&mut packet, &sk1).unwrap();

    assert!(verify_packet_signature(&packet, &vk2).is_err());
}

#[test]
fn signature_is_der_encoded() {
    let (sk, _) = make_key_pair();
    let mut packet = make_packet();
    sign_packet(&mut packet, &sk).unwrap();

    // DER-encoded ECDSA signatures start with 0x30 (SEQUENCE tag)
    assert_eq!(packet.signature[0], 0x30);
    // P-256 DER signatures are typically 70-72 bytes
    assert!(
        packet.signature.len() >= 68 && packet.signature.len() <= 73,
        "DER signature length {} outside expected range",
        packet.signature.len()
    );
}

#[test]
fn signing_does_not_alter_packet_content() {
    let (sk, _) = make_key_pair();
    let mut packet = make_packet();
    let original_bytes = marshal_deterministic(&{
        let mut p = packet.clone();
        p.signature = Vec::new();
        p
    });

    sign_packet(&mut packet, &sk).unwrap();

    // Strip signature and re-serialize — should match original
    let mut after = packet.clone();
    after.signature = Vec::new();
    let after_bytes = marshal_deterministic(&after);

    assert_eq!(original_bytes, after_bytes);
}

#[test]
fn double_sign_overwrites() {
    let (sk, vk) = make_key_pair();
    let mut packet = make_packet();

    sign_packet(&mut packet, &sk).unwrap();
    let first_sig = packet.signature.clone();

    sign_packet(&mut packet, &sk).unwrap();
    let second_sig = packet.signature.clone();

    // Both signatures should verify
    verify_packet_signature(&packet, &vk).unwrap();

    // Signatures may differ (ECDSA uses a nonce) but both are valid
    // Just ensure the second sign didn't break anything
    assert!(!first_sig.is_empty());
    assert!(!second_sig.is_empty());
}
