# frozen_string_literal: true

module Cordum
  module Cap
    module Validate
      # Validates a JobRequest. Raises ValidationError on failure.
      def self.job_request!(req)
        raise ValidationError.new('job_id', 'job_id is required') if req.job_id.nil? || req.job_id.empty?
        raise ValidationError.new('topic', 'topic is required') if req.topic.nil? || req.topic.empty?
      end

      # Validates a JobResult. Raises ValidationError on failure.
      def self.job_result!(res)
        raise ValidationError.new('job_id', 'job_id is required') if res.job_id.nil? || res.job_id.empty?
        raise ValidationError.new('worker_id', 'worker_id is required') if res.worker_id.nil? || res.worker_id.empty?

        status = res.status
        raise ValidationError.new('status', 'status is required') if status == :JOB_STATUS_UNSPECIFIED
      end

      # Validates a BusPacket envelope and its nested payload.
      # Raises ValidationError on failure.
      def self.bus_packet!(pkt)
        raise ValidationError.new('sender_id', 'sender_id is required') if pkt.sender_id.nil? || pkt.sender_id.empty?

        if pkt.protocol_version != 0 && pkt.protocol_version != DEFAULT_PROTOCOL_VERSION
          raise ValidationError.new('protocol_version',
            "unsupported protocol version: #{pkt.protocol_version}")
        end

        case pkt.payload
        when :job_request
          job_request!(pkt.job_request)
        when :job_result
          job_result!(pkt.job_result)
        when :heartbeat
          heartbeat!(pkt.heartbeat)
        when :job_progress
          job_progress!(pkt.job_progress)
        when :job_cancel
          job_cancel!(pkt.job_cancel)
        when :alert
          # alerts have no required fields beyond what's in the envelope
        when :handshake
          handshake!(pkt.handshake)
        end
      end

      # Validates a Heartbeat message.
      def self.heartbeat!(hb)
        raise ValidationError.new('worker_id', 'worker_id is required') if hb.worker_id.nil? || hb.worker_id.empty?
      end

      # Validates a JobProgress message.
      def self.job_progress!(progress)
        raise ValidationError.new('job_id', 'job_id is required') if progress.job_id.nil? || progress.job_id.empty?
        if progress.percent < 0 || progress.percent > 100
          raise ValidationError.new('percent', 'percent must be 0-100')
        end
      end

      # Validates a JobCancel message.
      def self.job_cancel!(cancel)
        raise ValidationError.new('job_id', 'job_id is required') if cancel.job_id.nil? || cancel.job_id.empty?
      end

      # Validates a Handshake message.
      def self.handshake!(hs)
        raise ValidationError.new('component_id', 'component_id is required') if hs.component_id.nil? || hs.component_id.empty?
      end
    end
  end
end
