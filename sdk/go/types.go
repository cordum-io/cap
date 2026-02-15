package capsdk

import (
	"time"
)

// DefaultSubject names for CAP. These match the transport profile in the spec.
const (
	SubjectSubmit        = "sys.job.submit"
	SubjectResult        = "sys.job.result"
	SubjectHeartbeat     = "sys.heartbeat"
	SubjectAlert         = "sys.alert"
	SubjectProgress      = "sys.job.progress"
	SubjectCancel        = "sys.job.cancel"
	SubjectDLQ           = "sys.job.dlq"
	SubjectWorkflowEvent = "sys.workflow.event"
	SubjectHandshake     = "sys.handshake"
)

// DefaultProtocolVersion is the current CAP wire version.
const DefaultProtocolVersion int32 = 1

// DefaultHeartbeatInterval is a recommended interval for emitting heartbeats.
const DefaultHeartbeatInterval = 5 * time.Second
