package io.cordum.cap;

public final class Subjects {
    public static final String SUBMIT = "sys.job.submit";
    public static final String RESULT = "sys.job.result";
    public static final String HEARTBEAT = "sys.heartbeat";
    public static final String ALERT = "sys.alert";
    public static final String PROGRESS = "sys.job.progress";
    public static final String CANCEL = "sys.job.cancel";
    public static final String DLQ = "sys.job.dlq";
    public static final String WORKFLOW_EVENT = "sys.workflow.event";
    public static final String HANDSHAKE = "sys.handshake";

    public static final int DEFAULT_PROTOCOL_VERSION = 1;
    public static final long DEFAULT_HEARTBEAT_INTERVAL_MS = 5000;

    private Subjects() {}
}
