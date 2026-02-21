# frozen_string_literal: true

module Cordum
  module Cap
    # Context passed through the middleware chain to the handler.
    class Context
      attr_reader :trace_id, :sender_id

      def initialize(trace_id:, sender_id:)
        @trace_id = trace_id
        @sender_id = sender_id
      end
    end

    # Rack-style middleware chain.
    # A middleware is a callable that takes a handler and returns a new handler.
    # A handler is a callable that takes (ctx, request) and returns a JobResult.
    module MiddlewareChain
      # Applies middlewares in FIFO order (first added = outermost wrapper).
      # Equivalent to: mw1(mw2(mw3(handler)))
      def self.apply(handler, middlewares)
        middlewares.reverse_each.reduce(handler) { |h, mw| mw.call(h) }
      end

      # Returns a logging middleware that measures handler execution time.
      def self.logging(logger: nil)
        lambda do |handler|
          lambda do |ctx, req|
            start = Process.clock_gettime(Process::CLOCK_MONOTONIC, :millisecond)
            result = handler.call(ctx, req)
            elapsed = Process.clock_gettime(Process::CLOCK_MONOTONIC, :millisecond) - start
            if logger
              logger.info("[CAP] job=#{req.job_id} trace=#{ctx.trace_id} duration=#{elapsed}ms")
            end
            result
          end
        end
      end
    end
  end
end
