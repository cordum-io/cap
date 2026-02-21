#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ErrorCode {
    Unspecified = 0,
    // Protocol errors (100-199)
    VersionMismatch = 100,
    MalformedPacket = 101,
    UnknownPayload = 102,
    SignatureInvalid = 103,
    SignatureMissing = 104,
    // Job errors (200-299)
    JobTimeout = 200,
    ResourceExhausted = 201,
    PermissionDenied = 202,
    InvalidInput = 203,
    JobNotFound = 204,
    DuplicateJob = 205,
    WorkerUnavailable = 206,
    // Safety errors (300-399)
    SafetyDenied = 300,
    PolicyViolation = 301,
    RiskTagBlocked = 302,
    // Transport errors (400-499)
    PublishFailed = 400,
    SubscribeFailed = 401,
    ConnectionLost = 402,
}

#[derive(Debug, thiserror::Error)]
pub enum CapError {
    #[error("protocol error ({code:?}): {message}")]
    Protocol { code: ErrorCode, message: String },

    #[error("validation error: field={field}, {message}")]
    Validation { field: String, message: String },

    #[error("signing error: {0}")]
    Signing(String),

    #[error("transport error: {0}")]
    Transport(String),
}

impl CapError {
    pub fn validation(field: &str, message: &str) -> Self {
        CapError::Validation {
            field: field.into(),
            message: message.into(),
        }
    }

    pub fn version_mismatch(msg: &str) -> Self {
        CapError::Protocol { code: ErrorCode::VersionMismatch, message: msg.into() }
    }

    pub fn malformed_packet(msg: &str) -> Self {
        CapError::Protocol { code: ErrorCode::MalformedPacket, message: msg.into() }
    }

    pub fn unknown_payload(msg: &str) -> Self {
        CapError::Protocol { code: ErrorCode::UnknownPayload, message: msg.into() }
    }

    pub fn signature_invalid(msg: &str) -> Self {
        CapError::Protocol { code: ErrorCode::SignatureInvalid, message: msg.into() }
    }

    pub fn signature_missing(msg: &str) -> Self {
        CapError::Protocol { code: ErrorCode::SignatureMissing, message: msg.into() }
    }

    pub fn job_timeout(msg: &str) -> Self {
        CapError::Protocol { code: ErrorCode::JobTimeout, message: msg.into() }
    }

    pub fn resource_exhausted(msg: &str) -> Self {
        CapError::Protocol { code: ErrorCode::ResourceExhausted, message: msg.into() }
    }

    pub fn permission_denied(msg: &str) -> Self {
        CapError::Protocol { code: ErrorCode::PermissionDenied, message: msg.into() }
    }

    pub fn invalid_input(msg: &str) -> Self {
        CapError::Protocol { code: ErrorCode::InvalidInput, message: msg.into() }
    }

    pub fn job_not_found(msg: &str) -> Self {
        CapError::Protocol { code: ErrorCode::JobNotFound, message: msg.into() }
    }

    pub fn duplicate_job(msg: &str) -> Self {
        CapError::Protocol { code: ErrorCode::DuplicateJob, message: msg.into() }
    }

    pub fn worker_unavailable(msg: &str) -> Self {
        CapError::Protocol { code: ErrorCode::WorkerUnavailable, message: msg.into() }
    }

    pub fn safety_denied(msg: &str) -> Self {
        CapError::Protocol { code: ErrorCode::SafetyDenied, message: msg.into() }
    }

    pub fn policy_violation(msg: &str) -> Self {
        CapError::Protocol { code: ErrorCode::PolicyViolation, message: msg.into() }
    }

    pub fn risk_tag_blocked(msg: &str) -> Self {
        CapError::Protocol { code: ErrorCode::RiskTagBlocked, message: msg.into() }
    }

    pub fn publish_failed(msg: &str) -> Self {
        CapError::Transport(format!("publish failed: {}", msg))
    }

    pub fn subscribe_failed(msg: &str) -> Self {
        CapError::Transport(format!("subscribe failed: {}", msg))
    }

    pub fn connection_lost(msg: &str) -> Self {
        CapError::Transport(format!("connection lost: {}", msg))
    }
}
