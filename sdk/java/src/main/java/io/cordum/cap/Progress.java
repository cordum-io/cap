package io.cordum.cap;

import com.google.protobuf.Timestamp;
import io.cordum.cap.agent.v1.BusPacket;
import io.cordum.cap.agent.v1.JobCancel;
import io.cordum.cap.agent.v1.JobProgress;
import io.nats.client.Connection;

public final class Progress {

    private Progress() {}

    public static byte[] progressPayload(String senderId, String jobId,
                                          String stepId, int percent, String message) {
        long now = System.currentTimeMillis();
        Timestamp ts = Timestamp.newBuilder()
                .setSeconds(now / 1000)
                .setNanos((int) ((now % 1000) * 1_000_000))
                .build();

        JobProgress jp = JobProgress.newBuilder()
                .setJobId(jobId)
                .setStepId(stepId)
                .setPercent(percent)
                .setMessage(message)
                .build();

        BusPacket packet = BusPacket.newBuilder()
                .setSenderId(senderId)
                .setProtocolVersion(Subjects.DEFAULT_PROTOCOL_VERSION)
                .setCreatedAt(ts)
                .setJobProgress(jp)
                .build();

        return Codec.marshalDeterministic(packet);
    }

    public static byte[] cancelPayload(String senderId, String jobId,
                                        String reason, String requestedBy) {
        long now = System.currentTimeMillis();
        Timestamp ts = Timestamp.newBuilder()
                .setSeconds(now / 1000)
                .setNanos((int) ((now % 1000) * 1_000_000))
                .build();

        JobCancel jc = JobCancel.newBuilder()
                .setJobId(jobId)
                .setReason(reason)
                .setRequestedBy(requestedBy)
                .build();

        BusPacket packet = BusPacket.newBuilder()
                .setSenderId(senderId)
                .setProtocolVersion(Subjects.DEFAULT_PROTOCOL_VERSION)
                .setCreatedAt(ts)
                .setJobCancel(jc)
                .build();

        return Codec.marshalDeterministic(packet);
    }

    public static void emitProgress(Connection nc, byte[] payload) {
        nc.publish(Subjects.PROGRESS, payload);
    }

    public static void emitCancel(Connection nc, byte[] payload) {
        nc.publish(Subjects.CANCEL, payload);
    }
}
