# frozen_string_literal: true

module Cordum
  module Cap
    module HeartbeatHelper
      # Builds a basic heartbeat payload (serialized BusPacket bytes).
      def self.heartbeat_payload(worker_id:, pool:, active_jobs:, max_parallel:, cpu_load:)
        heartbeat_payload_with_progress(
          worker_id: worker_id, pool: pool,
          active_jobs: active_jobs, max_parallel: max_parallel,
          cpu_load: cpu_load
        )
      end

      # Builds a heartbeat payload with memory load.
      def self.heartbeat_payload_with_memory(worker_id:, pool:, active_jobs:, max_parallel:, cpu_load:, memory_load:)
        heartbeat_payload_with_progress(
          worker_id: worker_id, pool: pool,
          active_jobs: active_jobs, max_parallel: max_parallel,
          cpu_load: cpu_load, memory_load: memory_load
        )
      end

      # Builds a heartbeat payload with all fields including progress.
      def self.heartbeat_payload_with_progress(worker_id:, pool:, active_jobs:, max_parallel:,
                                                cpu_load:, memory_load: 0.0, progress_pct: 0, last_memo: '')
        hb = Cordum::Agent::V1::Heartbeat.new(
          worker_id: worker_id,
          pool: pool,
          active_jobs: active_jobs,
          max_parallel_jobs: max_parallel,
          cpu_load: cpu_load.to_f,
          memory_load: memory_load.to_f,
          progress_pct: progress_pct,
          last_memo: last_memo
        )

        pkt = Cordum::Agent::V1::BusPacket.new(
          sender_id: worker_id,
          protocol_version: DEFAULT_PROTOCOL_VERSION,
          created_at: Google::Protobuf::Timestamp.new(seconds: Time.now.to_i, nanos: 0),
          heartbeat: hb
        )

        Codec.marshal_deterministic(pkt)
      end

      # Publishes a heartbeat payload to NATS.
      def self.emit(nc, payload)
        nc.publish(SUBJECT_HEARTBEAT, payload)
      end
    end

    # Background thread that periodically emits heartbeats.
    class HeartbeatLoop
      # @param nc [NATS::IO::Client] NATS connection
      # @param payload_fn [Proc] callable returning heartbeat payload bytes
      # @param interval [Numeric] seconds between heartbeats
      # @param metrics [MetricsHook, nil] optional metrics hook
      # @param worker_id [String] worker ID for metrics
      def initialize(nc:, payload_fn:, interval: DEFAULT_HEARTBEAT_INTERVAL, metrics: nil, worker_id: '')
        @nc = nc
        @payload_fn = payload_fn
        @interval = interval
        @metrics = metrics
        @worker_id = worker_id
        @running = false
        @thread = nil
      end

      # Starts the heartbeat loop in a background thread.
      def start
        @running = true
        @thread = Thread.new do
          while @running
            tick
            sleep @interval
          end
        end
        @thread.abort_on_exception = true
        self
      end

      # Executes a single heartbeat tick. Useful for external event loops.
      def tick
        payload = @payload_fn.call
        HeartbeatHelper.emit(@nc, payload)
        @metrics&.on_heartbeat_sent(@worker_id)
      rescue StandardError => e
        @metrics&.on_error('heartbeat', e.message)
      end

      # Stops the heartbeat loop and waits for the thread to finish.
      def stop
        @running = false
        @thread&.join(2)
      end

      def running?
        @running
      end
    end
  end
end
