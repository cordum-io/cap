package io.cordum.cap;

public class Errors {

    public enum ErrorCode {
        UNSPECIFIED(0),
        // Protocol errors (100-199)
        VERSION_MISMATCH(100),
        MALFORMED_PACKET(101),
        UNKNOWN_PAYLOAD(102),
        SIGNATURE_INVALID(103),
        SIGNATURE_MISSING(104),
        // Job errors (200-299)
        JOB_TIMEOUT(200),
        RESOURCE_EXHAUSTED(201),
        PERMISSION_DENIED(202),
        INVALID_INPUT(203),
        JOB_NOT_FOUND(204),
        DUPLICATE_JOB(205),
        WORKER_UNAVAILABLE(206),
        // Safety errors (300-399)
        SAFETY_DENIED(300),
        POLICY_VIOLATION(301),
        RISK_TAG_BLOCKED(302),
        // Transport errors (400-499)
        PUBLISH_FAILED(400),
        SUBSCRIBE_FAILED(401),
        CONNECTION_LOST(402);

        private final int code;

        ErrorCode(int code) {
            this.code = code;
        }

        public int code() {
            return code;
        }
    }

    public static class ProtocolException extends RuntimeException {
        private final ErrorCode code;

        public ProtocolException(ErrorCode code, String message) {
            super(message);
            this.code = code;
        }

        public ErrorCode code() {
            return code;
        }
    }

    public static class ValidationException extends RuntimeException {
        private final String field;

        public ValidationException(String field, String message) {
            super(field + ": " + message);
            this.field = field;
        }

        public String field() {
            return field;
        }
    }

    // Protocol error factories
    public static ProtocolException versionMismatch(String msg) {
        return new ProtocolException(ErrorCode.VERSION_MISMATCH, msg);
    }

    public static ProtocolException malformedPacket(String msg) {
        return new ProtocolException(ErrorCode.MALFORMED_PACKET, msg);
    }

    public static ProtocolException unknownPayload(String msg) {
        return new ProtocolException(ErrorCode.UNKNOWN_PAYLOAD, msg);
    }

    public static ProtocolException signatureInvalid(String msg) {
        return new ProtocolException(ErrorCode.SIGNATURE_INVALID, msg);
    }

    public static ProtocolException signatureMissing(String msg) {
        return new ProtocolException(ErrorCode.SIGNATURE_MISSING, msg);
    }

    // Job error factories
    public static ProtocolException jobTimeout(String msg) {
        return new ProtocolException(ErrorCode.JOB_TIMEOUT, msg);
    }

    public static ProtocolException resourceExhausted(String msg) {
        return new ProtocolException(ErrorCode.RESOURCE_EXHAUSTED, msg);
    }

    public static ProtocolException permissionDenied(String msg) {
        return new ProtocolException(ErrorCode.PERMISSION_DENIED, msg);
    }

    public static ProtocolException invalidInput(String msg) {
        return new ProtocolException(ErrorCode.INVALID_INPUT, msg);
    }

    public static ProtocolException jobNotFound(String msg) {
        return new ProtocolException(ErrorCode.JOB_NOT_FOUND, msg);
    }

    public static ProtocolException duplicateJob(String msg) {
        return new ProtocolException(ErrorCode.DUPLICATE_JOB, msg);
    }

    public static ProtocolException workerUnavailable(String msg) {
        return new ProtocolException(ErrorCode.WORKER_UNAVAILABLE, msg);
    }

    // Safety error factories
    public static ProtocolException safetyDenied(String msg) {
        return new ProtocolException(ErrorCode.SAFETY_DENIED, msg);
    }

    public static ProtocolException policyViolation(String msg) {
        return new ProtocolException(ErrorCode.POLICY_VIOLATION, msg);
    }

    public static ProtocolException riskTagBlocked(String msg) {
        return new ProtocolException(ErrorCode.RISK_TAG_BLOCKED, msg);
    }

    // Transport error factories
    public static ProtocolException publishFailed(String msg) {
        return new ProtocolException(ErrorCode.PUBLISH_FAILED, msg);
    }

    public static ProtocolException subscribeFailed(String msg) {
        return new ProtocolException(ErrorCode.SUBSCRIBE_FAILED, msg);
    }

    public static ProtocolException connectionLost(String msg) {
        return new ProtocolException(ErrorCode.CONNECTION_LOST, msg);
    }

    private Errors() {}
}
