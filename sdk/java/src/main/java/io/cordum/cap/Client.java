package io.cordum.cap;

import com.google.protobuf.Timestamp;
import io.cordum.cap.agent.v1.BusPacket;
import io.cordum.cap.agent.v1.JobRequest;
import io.nats.client.Connection;

import java.security.GeneralSecurityException;
import java.security.PrivateKey;
import java.security.PublicKey;
import java.util.Map;

public class Client {

    private final Connection nc;
    private final String senderId;
    private final PrivateKey privateKey;
    private final Map<String, PublicKey> publicKeys;

    public Client(Connection nc, String senderId) {
        this(nc, senderId, null, Map.of());
    }

    public Client(Connection nc, String senderId, PrivateKey privateKey,
                  Map<String, PublicKey> publicKeys) {
        this.nc = nc;
        this.senderId = senderId;
        this.privateKey = privateKey;
        this.publicKeys = publicKeys;
    }

    public void submit(String traceId, JobRequest req) {
        submit(traceId, req, Subjects.DEFAULT_PROTOCOL_VERSION);
    }

    public void submit(String traceId, JobRequest req, int protocolVersion) {
        long now = System.currentTimeMillis();
        Timestamp ts = Timestamp.newBuilder()
                .setSeconds(now / 1000)
                .setNanos((int) ((now % 1000) * 1_000_000))
                .build();

        BusPacket packet = BusPacket.newBuilder()
                .setTraceId(traceId)
                .setSenderId(senderId)
                .setProtocolVersion(protocolVersion)
                .setCreatedAt(ts)
                .setJobRequest(req)
                .build();

        if (privateKey != null) {
            try {
                packet = Signing.signPacket(packet, privateKey);
            } catch (GeneralSecurityException e) {
                throw Errors.publishFailed("signing failed: " + e.getMessage());
            }
        }

        byte[] data = Codec.marshalDeterministic(packet);
        nc.publish(Subjects.SUBMIT, data);
    }
}
