# frozen_string_literal: true

module Cordum
  module Cap
    class Agent
      # @param nats_url [String] NATS server URL
      # @param sender_id [String] this agent's sender ID
      # @param private_key [OpenSSL::PKey::EC, nil] optional signing key
      # @param public_keys [Hash{String => OpenSSL::PKey::EC}] sender_id => key map
      # @param metrics [MetricsHook] metrics callback object
      # @param heartbeat_interval [Numeric] seconds between heartbeats
      # @param pool [String] heartbeat pool name
      # @param max_parallel [Integer] advertised concurrency limit
      def initialize(nats_url:, sender_id:, private_key: nil, public_keys: {},
                     metrics: NoopMetrics.new, heartbeat_interval: DEFAULT_HEARTBEAT_INTERVAL,
                     pool: '', max_parallel: 1)
        @nats_url = nats_url
        @sender_id = sender_id
        @private_key = private_key
        @public_keys = public_keys
        @metrics = metrics
        @heartbeat_interval = heartbeat_interval
        @pool = pool
        @max_parallel = max_parallel
        @handlers = {}
        @middlewares = []
        @workers = []
        @heartbeat_loop = nil
        @nc = nil
        @active_jobs = 0
        @mutex = Mutex.new
      end

      # Registers a handler block for a topic.
      def register(topic, &handler)
        @handlers[topic] = handler
        self
      end

      # Adds middleware(s) to the processing chain.
      def use(*middlewares)
        @middlewares.concat(middlewares)
        self
      end

      # Connects to NATS, subscribes all registered handlers, starts heartbeat loop.
      def start
        @nc = Bus.connect(url: @nats_url, name: @sender_id)

        @handlers.each do |topic, handler|
          wrapped = MiddlewareChain.apply(handler, @middlewares)
          tracking_handler = build_tracking_handler(wrapped)

          worker = Worker.new(
            nc: @nc, subject: topic, sender_id: @sender_id,
            handler: tracking_handler,
            private_key: @private_key, public_keys: @public_keys,
            metrics: @metrics
          )
          @workers << worker
          Thread.new { worker.start }
        end

        start_heartbeat_loop
        self
      end

      # Stops all workers and the heartbeat loop, closes NATS.
      def close
        @heartbeat_loop&.stop
        @workers.each(&:stop)
        @nc&.close
      end

      private

      def build_tracking_handler(handler)
        lambda do |ctx, req|
          @mutex.synchronize { @active_jobs += 1 }
          begin
            handler.call(ctx, req)
          ensure
            @mutex.synchronize { @active_jobs -= 1 }
          end
        end
      end

      def start_heartbeat_loop
        payload_fn = lambda do
          active = @mutex.synchronize { @active_jobs }
          HeartbeatHelper.heartbeat_payload(
            worker_id: @sender_id,
            pool: @pool,
            active_jobs: active,
            max_parallel: @max_parallel,
            cpu_load: 0.0
          )
        end

        @heartbeat_loop = HeartbeatLoop.new(
          nc: @nc, payload_fn: payload_fn,
          interval: @heartbeat_interval,
          metrics: @metrics, worker_id: @sender_id
        )
        @heartbeat_loop.start
      end
    end
  end
end
