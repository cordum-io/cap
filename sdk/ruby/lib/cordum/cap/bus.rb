# frozen_string_literal: true

require 'nats/io/client'

module Cordum
  module Cap
    module Bus
      # Connects to a NATS server and returns the client.
      #
      # @param url [String] NATS server URL (e.g. "nats://localhost:4222")
      # @param token [String, nil] optional auth token
      # @param name [String, nil] optional client name
      # @return [NATS::IO::Client]
      def self.connect(url:, token: nil, name: nil)
        nc = NATS::IO::Client.new
        opts = { servers: [url] }
        opts[:token] = token if token
        opts[:name] = name if name
        nc.connect(opts)
        nc
      end
    end
  end
end
