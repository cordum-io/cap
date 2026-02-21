# frozen_string_literal: true

require 'openssl'

module Cordum
  module Cap
    module Signing
      # Signs a BusPacket in place, setting its signature field.
      # Returns the DER-encoded ECDSA signature bytes.
      def self.sign_packet(packet, private_key)
        unsigned_bytes = Codec.marshal_unsigned_for_signature(packet)
        digest = OpenSSL::Digest::SHA256.new
        signature = private_key.sign(digest, unsigned_bytes)
        packet.signature = signature
        signature
      end

      # Verifies the signature on a BusPacket against a public key.
      # Returns true if the signature is valid, false otherwise.
      def self.verify_packet_signature(packet, public_key)
        sig = packet.signature
        return false if sig.nil? || sig.empty?

        unsigned_bytes = Codec.marshal_unsigned_for_signature(packet)
        digest = OpenSSL::Digest::SHA256.new
        public_key.verify(digest, sig, unsigned_bytes)
      rescue OpenSSL::PKey::PKeyError
        false
      end

      # Loads an ECDSA private key from PEM-encoded string.
      def self.load_private_key(pem)
        OpenSSL::PKey::EC.new(pem)
      end

      # Loads an ECDSA public key from PEM-encoded string.
      def self.load_public_key(pem)
        OpenSSL::PKey::EC.new(pem)
      end

      # Generates a new ECDSA P-256 key pair for testing.
      # Returns [private_key, public_key].
      def self.generate_key_pair
        key = OpenSSL::PKey::EC.generate('prime256v1')
        pub = OpenSSL::PKey::EC.new(key.public_to_pem)
        [key, pub]
      end
    end
  end
end
