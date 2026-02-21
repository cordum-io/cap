# frozen_string_literal: true

module Cordum
  module Cap
    # Mock NATS client for testing without a real NATS server.
    class MockNats
      attr_reader :published

      def initialize
        @published = []
        @subscriptions = {}
      end

      def publish(subject, data)
        @published << [subject, data]
      end

      def subscribe(subject, **opts, &block)
        @subscriptions[subject] = { opts: opts, block: block }
        subject
      end

      def unsubscribe(_sid); end

      def close; end

      # Returns the number of published messages.
      def published_count
        @published.length
      end

      # Returns published messages filtered by subject.
      def published_to(subject)
        @published.select { |s, _| s == subject }
      end

      # Clears all published messages.
      def clear
        @published.clear
      end
    end

    # Test helper module for invoking handlers without NATS.
    module TestHelper
      # Submits a request to a handler and returns the result.
      #
      # @param handler [Proc] callable (ctx, request) -> JobResult
      # @param request [Cordum::Agent::V1::JobRequest]
      # @return [Cordum::Agent::V1::JobResult]
      def self.submit_and_wait(handler, request)
        ctx = Context.new(trace_id: 'test-trace', sender_id: 'test-sender')
        handler.call(ctx, request)
      end
    end

    # Metrics implementation that records all events for test assertions.
    class RecordingMetrics
      include MetricsHook

      attr_reader :received_jobs, :completed_jobs, :failed_jobs,
                  :heartbeats, :progress_events, :errors

      def initialize
        @received_jobs = []
        @completed_jobs = []
        @failed_jobs = []
        @heartbeats = []
        @progress_events = []
        @errors = []
      end

      def on_job_received(job_id, topic)
        @received_jobs << { job_id: job_id, topic: topic }
      end

      def on_job_completed(job_id, duration_ms, status)
        @completed_jobs << { job_id: job_id, duration_ms: duration_ms, status: status }
      end

      def on_job_failed(job_id, error_msg)
        @failed_jobs << { job_id: job_id, error_msg: error_msg }
      end

      def on_heartbeat_sent(worker_id)
        @heartbeats << { worker_id: worker_id }
      end

      def on_progress_emitted(job_id, percent)
        @progress_events << { job_id: job_id, percent: percent }
      end

      def on_error(category, message)
        @errors << "#{category}: #{message}"
      end
    end
  end
end
