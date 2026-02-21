# frozen_string_literal: true

module Cordum
  module Cap
    class Client
      # @param nc [NATS::IO::Client] connected NATS client
      # @param private_key [OpenSSL::PKey::EC, nil] optional signing key
      # @param public_keys [Hash{String => OpenSSL::PKey::EC}] sender_id => public key map
      def initialize(nc:, private_key: nil, public_keys: {})
        @nc = nc
        @private_key = private_key
        @public_keys = public_keys
      end

      # Submits a job request by wrapping it in a BusPacket and publishing.
      #
      # @param req [Cordum::Agent::V1::JobRequest]
      # @param trace_id [String]
      # @param sender_id [String]
      def submit(req, trace_id:, sender_id:)
        submit_with_version(req, trace_id: trace_id, sender_id: sender_id,
                            protocol_version: DEFAULT_PROTOCOL_VERSION)
      end

      # Submits a job request with an explicit protocol version.
      def submit_with_version(req, trace_id:, sender_id:, protocol_version:)
        pkt = Cordum::Agent::V1::BusPacket.new(
          trace_id: trace_id,
          sender_id: sender_id,
          protocol_version: protocol_version,
          created_at: Google::Protobuf::Timestamp.new(
            seconds: Time.now.to_i,
            nanos: 0
          ),
          job_request: req
        )

        Signing.sign_packet(pkt, @private_key) if @private_key

        data = Codec.marshal_deterministic(pkt)
        @nc.publish(SUBJECT_SUBMIT, data)
      end
    end
  end
end
