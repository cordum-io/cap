# frozen_string_literal: true

module Cordum
  module Cap
    # Error codes matching the proto ErrorCode enum ranges.
    module ErrorCodes
      # Protocol errors (100-104)
      VERSION_MISMATCH  = 100
      MALFORMED_PACKET  = 101
      UNKNOWN_PAYLOAD   = 102
      SIGNATURE_INVALID = 103
      SIGNATURE_MISSING = 104

      # Job errors (200-206)
      JOB_TIMEOUT            = 200
      JOB_RESOURCE_EXHAUSTED = 201
      JOB_PERMISSION_DENIED  = 202
      JOB_INVALID_INPUT      = 203
      JOB_NOT_FOUND          = 204
      JOB_DUPLICATE          = 205
      JOB_WORKER_UNAVAILABLE = 206

      # Safety errors (300-302)
      SAFETY_DENIED           = 300
      SAFETY_POLICY_VIOLATION = 301
      SAFETY_RISK_TAG_BLOCKED = 302

      # Transport errors (400-402)
      TRANSPORT_PUBLISH_FAILED   = 400
      TRANSPORT_SUBSCRIBE_FAILED = 401
      TRANSPORT_CONNECTION_LOST  = 402
    end

    # Base exception for all CAP SDK errors.
    class CapError < StandardError
      attr_reader :code

      def initialize(code, message)
        @code = code
        super(message)
      end
    end

    # Raised when input validation fails.
    class ValidationError < CapError
      attr_reader :field

      def initialize(field, message)
        @field = field
        super(ErrorCodes::JOB_INVALID_INPUT, message)
      end
    end

    # Factory methods for creating typed errors.
    module Errors
      def self.version_mismatch(msg)
        CapError.new(ErrorCodes::VERSION_MISMATCH, msg)
      end

      def self.malformed_packet(msg)
        CapError.new(ErrorCodes::MALFORMED_PACKET, msg)
      end

      def self.unknown_payload(msg)
        CapError.new(ErrorCodes::UNKNOWN_PAYLOAD, msg)
      end

      def self.signature_invalid(msg)
        CapError.new(ErrorCodes::SIGNATURE_INVALID, msg)
      end

      def self.signature_missing(msg)
        CapError.new(ErrorCodes::SIGNATURE_MISSING, msg)
      end

      def self.job_timeout(msg)
        CapError.new(ErrorCodes::JOB_TIMEOUT, msg)
      end

      def self.job_resource_exhausted(msg)
        CapError.new(ErrorCodes::JOB_RESOURCE_EXHAUSTED, msg)
      end

      def self.job_permission_denied(msg)
        CapError.new(ErrorCodes::JOB_PERMISSION_DENIED, msg)
      end

      def self.job_invalid_input(msg)
        CapError.new(ErrorCodes::JOB_INVALID_INPUT, msg)
      end

      def self.job_not_found(msg)
        CapError.new(ErrorCodes::JOB_NOT_FOUND, msg)
      end

      def self.job_duplicate(msg)
        CapError.new(ErrorCodes::JOB_DUPLICATE, msg)
      end

      def self.job_worker_unavailable(msg)
        CapError.new(ErrorCodes::JOB_WORKER_UNAVAILABLE, msg)
      end

      def self.safety_denied(msg)
        CapError.new(ErrorCodes::SAFETY_DENIED, msg)
      end

      def self.safety_policy_violation(msg)
        CapError.new(ErrorCodes::SAFETY_POLICY_VIOLATION, msg)
      end

      def self.safety_risk_tag_blocked(msg)
        CapError.new(ErrorCodes::SAFETY_RISK_TAG_BLOCKED, msg)
      end

      def self.transport_publish_failed(msg)
        CapError.new(ErrorCodes::TRANSPORT_PUBLISH_FAILED, msg)
      end

      def self.transport_subscribe_failed(msg)
        CapError.new(ErrorCodes::TRANSPORT_SUBSCRIBE_FAILED, msg)
      end

      def self.transport_connection_lost(msg)
        CapError.new(ErrorCodes::TRANSPORT_CONNECTION_LOST, msg)
      end
    end
  end
end
