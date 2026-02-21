# frozen_string_literal: true

module Cordum
  module Cap
    module Codec
      # Serializes a protobuf message deterministically.
      # Ruby's google-protobuf produces deterministic output for proto3 messages.
      def self.marshal_deterministic(msg)
        msg.to_proto
      end

      # Serializes a BusPacket with the signature field cleared,
      # producing the canonical bytes used for signing and verification.
      def self.marshal_unsigned_for_signature(packet)
        clone = packet.class.decode(packet.to_proto)
        clone.signature = ''.b
        marshal_deterministic(clone)
      end
    end
  end
end
