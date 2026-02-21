use std::fs;
use std::path::{Path, PathBuf};

use cap_sdk::codec::marshal_unsigned_for_signature;
use cap_sdk::pb::{bus_packet::Payload, BusPacket};
use cap_sdk::signing::load_public_key;
use p256::ecdsa::VerifyingKey;
use prost::Message;
use ecdsa::signature::Verifier;

fn fixtures_dir() -> PathBuf {
    let manifest_dir = Path::new(env!("CARGO_MANIFEST_DIR"));
    manifest_dir
        .parent()
        .unwrap()
        .parent()
        .unwrap()
        .join("spec")
        .join("conformance")
        .join("fixtures")
}

fn load_fixture(name: &str) -> BusPacket {
    let path = fixtures_dir().join(name);
    let data = fs::read(&path).unwrap_or_else(|e| panic!("read fixture {}: {}", name, e));
    BusPacket::decode(data.as_slice())
        .unwrap_or_else(|e| panic!("decode fixture {}: {}", name, e))
}

fn load_conformance_public_key() -> VerifyingKey {
    let path = fixtures_dir().join("public_key.pem");
    let pem = fs::read_to_string(&path).expect("read public_key.pem");
    load_public_key(&pem).expect("parse public key")
}

fn verify_fixture_signature(pkt: &BusPacket, key: &VerifyingKey) {
    assert!(
        !pkt.signature.is_empty(),
        "fixture packet has no signature"
    );
    cap_sdk::signing::verify_packet_signature(pkt, key)
        .expect("signature verification failed");
}

fn assert_timestamp(pkt: &BusPacket) {
    let ts = pkt.created_at.as_ref().expect("missing created_at");
    // Expected: 2024-01-02T03:04:05Z
    assert_eq!(ts.seconds, 1704164645, "unexpected created_at seconds");
    assert_eq!(ts.nanos, 0, "unexpected created_at nanos");
    assert_eq!(pkt.protocol_version, 1, "unexpected protocol_version");
}

#[test]
fn conformance_job_request() {
    let key = load_conformance_public_key();
    let pkt = load_fixture("buspacket_job_request.bin");

    verify_fixture_signature(&pkt, &key);
    assert_timestamp(&pkt);

    let req = match &pkt.payload {
        Some(Payload::JobRequest(r)) => r,
        _ => panic!("expected JobRequest payload"),
    };

    assert_eq!(req.job_id, "job-req-1");
    assert_eq!(req.topic, "job.tools");
    assert_eq!(req.context_ptr, "redis://ctx:job-req-1");
}

#[test]
fn conformance_job_result() {
    let key = load_conformance_public_key();
    let pkt = load_fixture("buspacket_job_result.bin");

    verify_fixture_signature(&pkt, &key);
    assert_timestamp(&pkt);

    let res = match &pkt.payload {
        Some(Payload::JobResult(r)) => r,
        _ => panic!("expected JobResult payload"),
    };

    assert_eq!(res.job_id, "job-res-1");
    assert_eq!(res.worker_id, "worker-1");
    assert_eq!(res.error_code, "E_TEMP");
    assert_eq!(res.artifact_ptrs.len(), 2);
}

#[test]
fn conformance_heartbeat() {
    let key = load_conformance_public_key();
    let pkt = load_fixture("buspacket_heartbeat.bin");

    verify_fixture_signature(&pkt, &key);
    assert_timestamp(&pkt);

    let hb = match &pkt.payload {
        Some(Payload::Heartbeat(h)) => h,
        _ => panic!("expected Heartbeat payload"),
    };

    assert_eq!(hb.worker_id, "worker-1");
    assert_eq!(hb.pool, "job.tools");
    assert_eq!(hb.labels.get("zone").map(String::as_str), Some("us-east-1a"));
}

#[test]
fn conformance_job_progress() {
    let key = load_conformance_public_key();
    let pkt = load_fixture("buspacket_job_progress.bin");

    verify_fixture_signature(&pkt, &key);
    assert_timestamp(&pkt);

    let progress = match &pkt.payload {
        Some(Payload::JobProgress(p)) => p,
        _ => panic!("expected JobProgress payload"),
    };

    assert_eq!(progress.job_id, "job-prog-1");
    assert_eq!(progress.percent, 50);
}

#[test]
fn conformance_job_cancel() {
    let key = load_conformance_public_key();
    let pkt = load_fixture("buspacket_job_cancel.bin");

    verify_fixture_signature(&pkt, &key);
    assert_timestamp(&pkt);

    let cancel = match &pkt.payload {
        Some(Payload::JobCancel(c)) => c,
        _ => panic!("expected JobCancel payload"),
    };

    assert_eq!(cancel.job_id, "job-cancel-1");
    assert_eq!(cancel.requested_by, "user-7");
}

#[test]
fn conformance_alert() {
    let key = load_conformance_public_key();
    let pkt = load_fixture("buspacket_alert.bin");

    verify_fixture_signature(&pkt, &key);
    assert_timestamp(&pkt);

    let alert = match &pkt.payload {
        Some(Payload::Alert(a)) => a,
        _ => panic!("expected Alert payload"),
    };

    assert_eq!(alert.level, "WARN");
    assert_eq!(alert.component, "scheduler");
}

#[test]
fn conformance_handshake() {
    let key = load_conformance_public_key();
    let pkt = load_fixture("buspacket_handshake.bin");

    verify_fixture_signature(&pkt, &key);
    assert_timestamp(&pkt);

    let hs = match &pkt.payload {
        Some(Payload::Handshake(h)) => h,
        _ => panic!("expected Handshake payload"),
    };

    assert_eq!(hs.component_id, "worker-1");
    assert_eq!(hs.supported_versions, vec![1]);
    assert_eq!(hs.sdk_version, "2.0.19");
    assert_eq!(hs.capabilities.get("signatures"), Some(&true));
    assert_eq!(hs.capabilities.get("progress"), Some(&true));
    assert_eq!(hs.capabilities.get("cancel"), Some(&true));
}

#[test]
fn conformance_alert_enhanced() {
    let key = load_conformance_public_key();
    let pkt = load_fixture("buspacket_alert_enhanced.bin");

    verify_fixture_signature(&pkt, &key);
    assert_timestamp(&pkt);

    let alert = match &pkt.payload {
        Some(Payload::Alert(a)) => a,
        _ => panic!("expected Alert payload"),
    };

    assert_eq!(alert.level, "CRITICAL");
    assert_eq!(alert.component, "scheduler");
    assert_eq!(alert.code, "SIGNATURE_INVALID");
    assert_eq!(alert.source_component, "scheduler-1");
    assert_eq!(
        alert.details.get("sender").map(String::as_str),
        Some("worker-bad")
    );
    assert_eq!(
        alert.details.get("subject").map(String::as_str),
        Some("sys.job.result")
    );
    assert_eq!(alert.trace_id, "trace-offending-packet");
}

#[test]
fn conformance_unsigned_bytes_match_across_fixtures() {
    let key = load_conformance_public_key();

    // For each fixture, verify that stripping signature and re-serializing
    // produces bytes whose SHA-256 hash can be verified against the signature
    let fixtures = [
        "buspacket_job_request.bin",
        "buspacket_job_result.bin",
        "buspacket_heartbeat.bin",
        "buspacket_job_progress.bin",
        "buspacket_job_cancel.bin",
        "buspacket_alert.bin",
        "buspacket_handshake.bin",
        "buspacket_alert_enhanced.bin",
    ];

    for fixture_name in &fixtures {
        let pkt = load_fixture(fixture_name);
        assert!(
            !pkt.signature.is_empty(),
            "{} has no signature",
            fixture_name
        );

        // Verify the full roundtrip: unsigned bytes -> verify with public key
        // VerifyingKey::verify() internally hashes with SHA-256 before verifying
        let unsigned_bytes = marshal_unsigned_for_signature(&pkt);

        let sig = p256::ecdsa::DerSignature::try_from(pkt.signature.as_slice())
            .unwrap_or_else(|e| panic!("{} DER parse: {}", fixture_name, e));

        key.verify(&unsigned_bytes, &sig)
            .unwrap_or_else(|e| panic!("{} verify: {}", fixture_name, e));
    }
}
