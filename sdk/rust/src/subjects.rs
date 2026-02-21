use std::time::Duration;

pub const SUBJECT_SUBMIT: &str = "sys.job.submit";
pub const SUBJECT_RESULT: &str = "sys.job.result";
pub const SUBJECT_HEARTBEAT: &str = "sys.heartbeat";
pub const SUBJECT_ALERT: &str = "sys.alert";
pub const SUBJECT_PROGRESS: &str = "sys.job.progress";
pub const SUBJECT_CANCEL: &str = "sys.job.cancel";
pub const SUBJECT_DLQ: &str = "sys.job.dlq";
pub const SUBJECT_WORKFLOW_EVENT: &str = "sys.workflow.event";
pub const SUBJECT_HANDSHAKE: &str = "sys.handshake";

pub const DEFAULT_PROTOCOL_VERSION: i32 = 1;
pub const DEFAULT_HEARTBEAT_INTERVAL: Duration = Duration::from_secs(5);
