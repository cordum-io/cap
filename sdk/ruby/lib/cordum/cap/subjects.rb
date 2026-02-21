# frozen_string_literal: true

module Cordum
  module Cap
    SUBJECT_SUBMIT         = 'sys.job.submit'
    SUBJECT_RESULT         = 'sys.job.result'
    SUBJECT_HEARTBEAT      = 'sys.heartbeat'
    SUBJECT_PROGRESS       = 'sys.job.progress'
    SUBJECT_CANCEL         = 'sys.job.cancel'
    SUBJECT_ALERT          = 'sys.alert'
    SUBJECT_DLQ            = 'sys.job.dlq'
    SUBJECT_WORKFLOW_EVENT = 'sys.workflow.event'
    SUBJECT_HANDSHAKE      = 'sys.handshake'

    DEFAULT_PROTOCOL_VERSION   = 1
    DEFAULT_HEARTBEAT_INTERVAL = 5
  end
end
