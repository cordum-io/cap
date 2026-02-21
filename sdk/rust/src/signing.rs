use ecdsa::signature::{Signer, Verifier};
use p256::ecdsa::{DerSignature, SigningKey, VerifyingKey};

use crate::codec::marshal_unsigned_for_signature;
use crate::errors::CapError;
use crate::pb::BusPacket;

/// Sign a BusPacket using ECDSA P-256 with SHA-256.
/// Sets the signature field on the packet in ASN.1/DER format.
///
/// The `p256::ecdsa::SigningKey` implements `Signer<DerSignature>` which
/// internally hashes the input with SHA-256 before signing. We pass the raw
/// unsigned bytes so the final operation is ECDSA_sign(SHA-256(unsigned)).
pub fn sign_packet(packet: &mut BusPacket, key: &SigningKey) -> Result<(), CapError> {
    let unsigned_bytes = marshal_unsigned_for_signature(packet);
    let signature: DerSignature = key.sign(&unsigned_bytes);
    packet.signature = signature.to_bytes().to_vec();
    Ok(())
}

/// Verify a BusPacket's ECDSA P-256 signature.
/// Returns Ok(()) if the signature is valid.
///
/// The `p256::ecdsa::VerifyingKey` implements `Verifier<DerSignature>` which
/// internally hashes the input with SHA-256 before verifying. We pass the raw
/// unsigned bytes so the final operation is ECDSA_verify(SHA-256(unsigned)).
pub fn verify_packet_signature(
    packet: &BusPacket,
    key: &VerifyingKey,
) -> Result<(), CapError> {
    if packet.signature.is_empty() {
        return Err(CapError::signature_missing("packet has no signature"));
    }

    let sig = DerSignature::try_from(packet.signature.as_slice())
        .map_err(|e| CapError::signature_invalid(&format!("invalid DER signature: {}", e)))?;

    let unsigned_bytes = marshal_unsigned_for_signature(packet);

    key.verify(&unsigned_bytes, &sig)
        .map_err(|e| CapError::signature_invalid(&format!("verification failed: {}", e)))
}

/// Load a PEM-encoded PKCS8 EC private key.
pub fn load_private_key(pem: &str) -> Result<SigningKey, CapError> {
    use p256::pkcs8::DecodePrivateKey;
    SigningKey::from_pkcs8_pem(pem)
        .map_err(|e| CapError::Signing(format!("failed to load private key: {}", e)))
}

/// Load a PEM-encoded SPKI EC public key.
pub fn load_public_key(pem: &str) -> Result<VerifyingKey, CapError> {
    use p256::pkcs8::DecodePublicKey;
    VerifyingKey::from_public_key_pem(pem)
        .map_err(|e| CapError::Signing(format!("failed to load public key: {}", e)))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::pb::{bus_packet::Payload, JobRequest};
    use p256::ecdsa::SigningKey;
    use rand_core::OsRng;

    fn make_packet() -> BusPacket {
        BusPacket {
            trace_id: "trace-1".into(),
            sender_id: "sender-1".into(),
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
    fn sign_and_verify_round_trip() {
        let key = SigningKey::random(&mut OsRng);
        let verifying_key = VerifyingKey::from(&key);

        let mut packet = make_packet();
        sign_packet(&mut packet, &key).unwrap();
        assert!(!packet.signature.is_empty());
        verify_packet_signature(&packet, &verifying_key).unwrap();
    }

    #[test]
    fn tamper_detection() {
        let key = SigningKey::random(&mut OsRng);
        let verifying_key = VerifyingKey::from(&key);

        let mut packet = make_packet();
        sign_packet(&mut packet, &key).unwrap();
        packet.sender_id = "tampered".into();

        assert!(verify_packet_signature(&packet, &verifying_key).is_err());
    }

    #[test]
    fn empty_signature_rejected() {
        let key = SigningKey::random(&mut OsRng);
        let verifying_key = VerifyingKey::from(&key);
        let packet = make_packet();
        assert!(verify_packet_signature(&packet, &verifying_key).is_err());
    }

    #[test]
    fn wrong_key_rejected() {
        let key1 = SigningKey::random(&mut OsRng);
        let key2 = SigningKey::random(&mut OsRng);
        let verifying_key2 = VerifyingKey::from(&key2);

        let mut packet = make_packet();
        sign_packet(&mut packet, &key1).unwrap();
        assert!(verify_packet_signature(&packet, &verifying_key2).is_err());
    }
}
