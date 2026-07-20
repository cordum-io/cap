use prost::Message;

use crate::pb::{bus_packet::Payload, BusPacket, Handshake};

/// Serialize a protobuf message deterministically.
/// prost encodes fields in field-number order, and build.rs configures all
/// generated protobuf maps as BTreeMap so entries are ordered by key.
pub fn marshal_deterministic(msg: &impl Message) -> Vec<u8> {
    msg.encode_to_vec()
}

/// Serialize a BusPacket with the signature field cleared.
/// Used as the input to ECDSA signing/verification.
pub fn marshal_unsigned_for_signature(packet: &BusPacket) -> Vec<u8> {
    let mut clone = packet.clone();
    clone.signature = Vec::new();
    if let Some(Payload::Handshake(handshake)) = &clone.payload {
        return marshal_legacy_handshake_packet(&clone, handshake);
    }
    marshal_deterministic(&clone)
}

fn marshal_legacy_handshake_packet(packet: &BusPacket, handshake: &Handshake) -> Vec<u8> {
    CanonicalHandshakePacket {
        trace_id: packet.trace_id.clone(),
        sender_id: packet.sender_id.clone(),
        created_at: packet.created_at,
        protocol_version: packet.protocol_version,
        handshake: Some(CanonicalHandshake::from(handshake)),
        auth_token: packet.auth_token.clone(),
    }
    .encode_to_vec()
}

#[derive(Clone, PartialEq, Message)]
struct CanonicalHandshakePacket {
    #[prost(string, tag = "1")]
    trace_id: String,
    #[prost(string, tag = "2")]
    sender_id: String,
    #[prost(message, optional, tag = "3")]
    created_at: Option<prost_types::Timestamp>,
    #[prost(int32, tag = "4")]
    protocol_version: i32,
    #[prost(message, optional, tag = "17")]
    handshake: Option<CanonicalHandshake>,
    #[prost(string, tag = "18")]
    auth_token: String,
}

#[derive(Clone, PartialEq, Message)]
struct CanonicalHandshake {
    #[prost(string, tag = "1")]
    component_id: String,
    #[prost(int32, tag = "2")]
    role: i32,
    #[prost(int32, repeated, packed = "true", tag = "3")]
    supported_versions: Vec<i32>,
    #[prost(message, repeated, tag = "4")]
    capabilities: Vec<CanonicalBoolMapEntry>,
    #[prost(string, tag = "5")]
    sdk_version: String,
    #[prost(string, repeated, tag = "6")]
    ready_topics: Vec<String>,
    #[prost(string, tag = "7")]
    agent_name: String,
}

impl From<&Handshake> for CanonicalHandshake {
    fn from(value: &Handshake) -> Self {
        let mut capabilities = value
            .capabilities
            .iter()
            .map(|(key, enabled)| CanonicalBoolMapEntry {
                key: key.clone(),
                value: Some(*enabled),
            })
            .collect::<Vec<_>>();
        capabilities.sort_by(|left, right| left.key.cmp(&right.key));
        Self {
            component_id: value.component_id.clone(),
            role: value.role,
            supported_versions: value.supported_versions.clone(),
            capabilities,
            sdk_version: value.sdk_version.clone(),
            ready_topics: value.ready_topics.clone(),
            agent_name: value.agent_name.clone(),
        }
    }
}

#[derive(Clone, PartialEq, Message)]
struct CanonicalBoolMapEntry {
    #[prost(string, tag = "1")]
    key: String,
    #[prost(bool, optional, tag = "2")]
    value: Option<bool>,
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::pb::{bus_packet::Payload, JobRequest};
    use crate::signing::{load_public_key, verify_packet_signature};

    fn make_packet() -> BusPacket {
        BusPacket {
            trace_id: "test-trace".into(),
            sender_id: "test-sender".into(),
            protocol_version: 1,
            payload: Some(Payload::JobRequest(JobRequest {
                job_id: "job-1".into(),
                topic: "test.topic".into(),
                ..Default::default()
            })),
            ..Default::default()
        }
    }

    #[test]
    fn deterministic_serialization_is_stable() {
        let packet = make_packet();
        let first = marshal_deterministic(&packet);
        assert!(!first.is_empty());

        for _ in 0..100 {
            assert_eq!(first, marshal_deterministic(&packet));
        }
    }

    #[test]
    fn marshal_unsigned_strips_signature() {
        let mut packet = make_packet();
        packet.signature = b"fake-sig".to_vec();

        let bytes = marshal_unsigned_for_signature(&packet);
        let decoded = BusPacket::decode(bytes.as_slice()).unwrap();
        assert!(decoded.signature.is_empty());
        assert_eq!(decoded.sender_id, "test-sender");
    }

    #[test]
    fn marshal_unsigned_preserves_all_fields() {
        let mut packet = make_packet();
        packet.signature = b"to-strip".to_vec();

        let without_sig = marshal_unsigned_for_signature(&packet);

        let mut clean = packet.clone();
        clean.signature = Vec::new();
        let clean_bytes = marshal_deterministic(&clean);

        assert_eq!(without_sig, clean_bytes);
    }

    #[test]
    fn empty_message() {
        let empty = BusPacket::default();
        let bytes = marshal_deterministic(&empty);
        assert!(bytes.is_empty());
    }

    #[test]
    fn round_trip() {
        let packet = make_packet();
        let bytes = marshal_deterministic(&packet);
        let decoded = BusPacket::decode(bytes.as_slice()).unwrap();
        assert_eq!(decoded.trace_id, packet.trace_id);
        assert_eq!(decoded.sender_id, packet.sender_id);
    }

    #[test]
    fn false_handshake_capability_matches_published_signature() {
        let packet = BusPacket::decode(
            include_bytes!("../../../spec/conformance/fixtures/buspacket_handshake.bin").as_slice(),
        )
        .unwrap();
        let handshake = match &packet.payload {
            Some(Payload::Handshake(value)) => value,
            _ => panic!("expected handshake fixture"),
        };
        assert_eq!(handshake.capabilities.get("compensation"), Some(&false));

        let key = load_public_key(include_str!(
            "../../../spec/conformance/fixtures/public_key.pem"
        ))
        .unwrap();
        verify_packet_signature(&packet, &key).unwrap();
    }
}
