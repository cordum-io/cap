package io.cordum.cap;

public interface Metrics {

    void onJobReceived(String jobId, String topic);

    void onJobCompleted(String jobId, long durationMs, String status);

    void onJobFailed(String jobId, String errorMsg);

    void onHeartbeatSent(String workerId);

    void onProgressEmitted(String jobId, int percent);

    void onError(String context, String errorMsg);

    Metrics NOOP = new Metrics() {
        @Override public void onJobReceived(String jobId, String topic) {}
        @Override public void onJobCompleted(String jobId, long durationMs, String status) {}
        @Override public void onJobFailed(String jobId, String errorMsg) {}
        @Override public void onHeartbeatSent(String workerId) {}
        @Override public void onProgressEmitted(String jobId, int percent) {}
        @Override public void onError(String context, String errorMsg) {}
    };
}
