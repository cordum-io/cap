# frozen_string_literal: true

module Cordum
  module Cap
    class Worker
      # @param nc [NATS::IO::Client] connected NATS client
      # @param subject [String] NATS subject to subscribe to
      # @param sender_id [String] this worker's sender ID
      # @param handler [Proc] callable (ctx, request) -> JobResult
      # @param private_key [OpenSSL::PKey::EC, nil] optional signing key
      # @param public_keys [Hash{String => OpenSSL::PKey::EC}] sender_id => key map
      # @param metrics [MetricsHook] metrics callback object
      # @param queue_group [String] NATS queue group for load balancing
      def initialize(nc:, subject:, sender_id:, handler:,
                     private_key: nil, public_keys: {}, metrics: NoopMetrics.new,
                     queue_group: 'workers')
        @nc = nc
        @subject = subject
        @sender_id = sender_id
        @handler = handler
        @private_key = private_key
        @public_keys = public_keys
        @metrics = metrics
        @queue_group = queue_group
        @middlewares = []
        @running = false
      end

      # Adds middleware(s) to the processing chain.
      def use(*middlewares)
        @middlewares.concat(middlewares)
        self
      end

      # Starts the worker subscription. Blocks until stop is called.
      def start
        wrapped = MiddlewareChain.apply(@handler, @middlewares)
        @running = true

        @sid = @nc.subscribe(@subject, queue: @queue_group) do |msg|
          process_message(msg.data, wrapped)
        end

        while @running
          sleep 0.1
        end
      end

      # Stops the worker.
      def stop
        @running = false
        @nc.unsubscribe(@sid) if @sid
      end

      private

      def process_message(data, handler)
        pkt = Cordum::Agent::V1::BusPacket.decode(data)

        # Verify signature if public keys are configured
        if !@public_keys.empty? && !pkt.sender_id.empty?
          pub_key = @public_keys[pkt.sender_id]
          if pub_key && !Signing.verify_packet_signature(pkt, pub_key)
            @metrics.on_error('signature', "invalid signature from #{pkt.sender_id}")
            return
          end
        end

        return unless pkt.payload == :job_request

        req = pkt.job_request
        @metrics.on_job_received(req.job_id, req.topic)

        ctx = Context.new(trace_id: pkt.trace_id, sender_id: pkt.sender_id)
        start_time = Process.clock_gettime(Process::CLOCK_MONOTONIC, :millisecond)

        begin
          result = handler.call(ctx, req)
          elapsed = Process.clock_gettime(Process::CLOCK_MONOTONIC, :millisecond) - start_time
          @metrics.on_job_completed(req.job_id, elapsed, result.status.to_s)
        rescue StandardError => e
          elapsed = Process.clock_gettime(Process::CLOCK_MONOTONIC, :millisecond) - start_time
          @metrics.on_job_failed(req.job_id, e.message)
          result = Cordum::Agent::V1::JobResult.new(
            job_id: req.job_id,
            worker_id: @sender_id,
            status: :JOB_STATUS_FAILED,
            error_code: e.is_a?(CapError) ? e.code.to_s : 'UNKNOWN',
            error_message: e.message,
            execution_ms: elapsed
          )
        end

        reply_pkt = Cordum::Agent::V1::BusPacket.new(
          trace_id: pkt.trace_id,
          sender_id: @sender_id,
          protocol_version: DEFAULT_PROTOCOL_VERSION,
          created_at: Google::Protobuf::Timestamp.new(seconds: Time.now.to_i, nanos: 0),
          job_result: result
        )

        Signing.sign_packet(reply_pkt, @private_key) if @private_key

        reply_data = Codec.marshal_deterministic(reply_pkt)
        @nc.publish(SUBJECT_RESULT, reply_data)
      rescue StandardError => e
        @metrics.on_error('process', e.message)
      end
    end
  end
end
