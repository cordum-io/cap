use prost::Message;

use crate::pb::BusPacket;

/// Serialize a protobuf message deterministically.
/// prost encodes fields in field-number order, which is deterministic for proto3
/// messages. For map fields, prost uses HashMap which is non-deterministic;
/// callers with map fields should use sorted construction or BTreeMap-backed types.
pub fn marshal_deterministic(msg: &impl Message) -> Vec<u8> {
    msg.encode_to_vec()
}

/// Serialize a BusPacket with the signature field cleared.
/// Used as the input to ECDSA signing/verification.
pub fn marshal_unsigned_for_signature(packet: &BusPacket) -> Vec<u8> {
    let mut clone = packet.clone();
    clone.signature = Vec::new();
    marshal_deterministic(&clone)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::pb::{bus_packet::Payload, JobRequest};

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
}
