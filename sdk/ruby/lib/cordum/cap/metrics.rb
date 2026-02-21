# frozen_string_literal: true

module Cordum
  module Cap
    # Mixin module for metrics callbacks. Include in your class and
    # override the methods you care about.
    module MetricsHook
      def on_job_received(job_id, topic); end
      def on_job_completed(job_id, duration_ms, status); end
      def on_job_failed(job_id, error_msg); end
      def on_heartbeat_sent(worker_id); end
      def on_progress_emitted(job_id, percent); end
      def on_error(category, message); end
    end

    # Default no-op metrics implementation.
    class NoopMetrics
      include MetricsHook
    end
  end
end
