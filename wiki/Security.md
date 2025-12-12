# Security Best Practices

This document provides a comprehensive overview of security best practices for implementing and deploying the coretex Agent Protocol (CAP). These recommendations are intended to supplement the baseline security requirements outlined in `10-security-observability.md`.

## Threat Model

A detailed threat model is crucial for securing any distributed system. The following is a non-exhaustive list of potential threats to a CAP deployment and recommended mitigations.

| Threat | Description | Mitigation |
| --- | --- | --- |
| **Message Spoofing** | A malicious actor on the bus impersonates a legitimate component (e.g., a worker spoofing a scheduler). | Use digital signatures for `BusPacket` envelopes. |
| **Message Tampering** | A malicious actor modifies a message in transit. | Use digital signatures for `BusPacket` envelopes and TLS for the transport layer. |
| **Unauthorized Job Execution** | A malicious actor submits a job they are not authorized to run. | Gateways MUST authenticate and authorize clients. Schedulers MUST verify job provenance. |
| **Data Exfiltration** | A malicious actor gains access to sensitive data in the memory fabric. | Use end-to-end encryption for payloads. Enforce strict access control on the memory fabric. |
| **Denial of Service (DoS)** | A malicious actor floods the system with jobs or heartbeats, overwhelming components. | Implement rate limiting at the gateway and on the bus. Schedulers should monitor for resource abuse. |
| **Malicious Payloads** | A malicious actor submits a job with a payload designed to exploit a vulnerability in a worker. | Gateways and workers MUST validate and sanitize inputs. Run workers in sandboxed environments. |
| **Component Compromise** | A legitimate component (e.g., a worker) is compromised and begins to act maliciously. | Implement the principle of least privilege. Workers should have limited access to the bus and memory fabric. Monitor for anomalous behavior. |

## End-to-End Encryption of Payloads

While transport-level encryption (TLS) protects data in transit on the bus, the payloads stored in the memory fabric are not encrypted by the protocol itself. For sensitive data, we strongly recommend end-to-end encryption of payloads.

- **Mechanism:** Before writing a payload to the memory fabric, the client or gateway should encrypt it using a symmetric key. The key should be managed by a Key Management System (KMS). The `context_ptr` would then point to the encrypted payload. The worker would fetch the encrypted payload, retrieve the key from the KMS, and decrypt the payload.
- **Key Management:** Use a KMS (e.g., HashiCorp Vault, AWS KMS, Google Cloud KMS) to manage encryption keys. Access to keys should be strictly controlled and audited.
- **Key Scoping:** Keys should be scoped as narrowly as possible, for example, per-tenant or per-job.

## Digital Signatures for BusPackets

To prevent message spoofing and tampering, all `BusPacket` envelopes SHOULD be digitally signed.

- **Mechanism:** The `BusPacket` proto can be extended to include a `signature` field. The sender would serialize the `BusPacket` (without the signature field), sign the serialized data with its private key, and then add the signature to the `signature` field. The receiver would verify the signature using the sender's public key.

```proto
message BusPacket {
  string trace_id = 1;
  string sender_id = 2;
  google.protobuf.Timestamp created_at = 3;
  int32 protocol_version = 4;

  oneof payload {
    JobRequest job_request = 10;
    JobResult job_result = 11;
    Heartbeat heartbeat = 12;
    SystemAlert alert = 13;
  }
  
  bytes signature = 14; // Digital signature of the serialized BusPacket
}
```

- **Public Key Distribution:** Public keys of trusted components must be distributed securely. This can be done through a configuration management system or a dedicated key distribution service.

## Detailed Safety Kernel Interaction

The Safety Kernel is a critical component for policy enforcement. To enable more sophisticated policies, the `PolicyCheckRequest` should include as much context as possible.

- **Recommended `PolicyCheckRequest` Fields:**
  - `job_id`, `topic`, `tenant`, `priority`, `adapter_id`, `estimated_cost`
  - `user_id`, `client_ip`
  - A hash of the `context_ptr` data, to allow the Safety Kernel to check for known malicious inputs without having access to the raw data.
  - `env` metadata.

## Input Validation

All inputs from external sources must be validated and sanitized.

- **Gateway:**
  - Validate the format of the `context_ptr` to ensure it's a valid URI for the configured memory fabric.
  - Sanitize all fields in the `JobRequest` to prevent injection attacks (e.g., XSS, SQLi) if these fields are used to construct queries or displayed in UIs.
  - Enforce size limits on `env` metadata.
- **Worker:**
  - Never trust the content of the `context_ptr`.
  - If the payload is in a structured format (e.g., JSON), validate it against a schema.
  - If the payload is executable code, run it in a sandboxed environment.

## Rate Limiting and Resource Abuse

To protect the system from being overwhelmed, implement rate limiting at multiple levels.

- **Gateway:**
  - Limit the number of job submissions per client / per IP address.
- **Bus:**
  - If the bus supports it, configure rate limits on subjects.
- **Scheduler:**
  - Monitor the number of jobs per tenant and enforce quotas.
  - Monitor worker heartbeats and detect anomalous behavior (e.g., a worker that is always busy).

## Principle of Least Privilege

Components should only have the permissions they need to perform their function.

- **Workers:** A worker should only be able to subscribe to its designated pool subject and publish to the `sys.job.result` subject. It should only have access to the specific memory locations for the jobs it's processing.
- **Gateways:** A gateway should only be able to publish to the `sys.job.submit` subject.
- **Schedulers:** A scheduler needs broader permissions, but these should still be carefully scoped.
