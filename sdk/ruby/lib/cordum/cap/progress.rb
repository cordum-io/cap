# frozen_string_literal: true

module Cordum
  module Cap
    module ProgressHelper
      # Builds a job progress payload (serialized BusPacket bytes).
      def self.progress_payload(sender_id:, job_id:, step_id:, percent:, message:)
        progress = Cordum::Agent::V1::JobProgress.new(
          job_id: job_id,
          step_id: step_id,
          percent: percent,
          message: message,
          status: :JOB_STATUS_RUNNING
        )

        pkt = Cordum::Agent::V1::BusPacket.new(
          sender_id: sender_id,
          protocol_version: DEFAULT_PROTOCOL_VERSION,
          created_at: Google::Protobuf::Timestamp.new(seconds: Time.now.to_i, nanos: 0),
          job_progress: progress
        )

        Codec.marshal_deterministic(pkt)
      end

      # Builds a job cancel payload (serialized BusPacket bytes).
      def self.cancel_payload(sender_id:, job_id:, reason:, requested_by:)
        cancel = Cordum::Agent::V1::JobCancel.new(
          job_id: job_id,
          reason: reason,
          requested_by: requested_by
        )

        pkt = Cordum::Agent::V1::BusPacket.new(
          sender_id: sender_id,
          protocol_version: DEFAULT_PROTOCOL_VERSION,
          created_at: Google::Protobuf::Timestamp.new(seconds: Time.now.to_i, nanos: 0),
          job_cancel: cancel
        )

        Codec.marshal_deterministic(pkt)
      end

      # Publishes a progress payload to NATS.
      def self.emit_progress(nc, payload)
        nc.publish(SUBJECT_PROGRESS, payload)
      end

      # Publishes a cancel payload to NATS.
      def self.emit_cancel(nc, payload)
        nc.publish(SUBJECT_CANCEL, payload)
      end
    end
  end
end
