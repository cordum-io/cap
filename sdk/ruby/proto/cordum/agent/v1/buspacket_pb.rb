# frozen_string_literal: true
# Generated from cordum/agent/v1/buspacket.proto — DO NOT EDIT

require 'google/protobuf'
require 'google/protobuf/timestamp_pb'
require_relative 'job_pb'
require_relative 'heartbeat_pb'
require_relative 'alert_pb'
require_relative 'handshake_pb'

Google::Protobuf::DescriptorPool.generated_pool.build do
  add_file('cordum/agent/v1/buspacket.proto', syntax: :proto3) do
    add_message 'cordum.agent.v1.BusPacket' do
      optional :trace_id, :string, 1
      optional :sender_id, :string, 2
      optional :created_at, :message, 3, 'google.protobuf.Timestamp'
      optional :protocol_version, :int32, 4
      oneof :payload do
        optional :job_request, :message, 10, 'cordum.agent.v1.JobRequest'
        optional :job_result, :message, 11, 'cordum.agent.v1.JobResult'
        optional :heartbeat, :message, 12, 'cordum.agent.v1.Heartbeat'
        optional :alert, :message, 13, 'cordum.agent.v1.SystemAlert'
        optional :job_progress, :message, 15, 'cordum.agent.v1.JobProgress'
        optional :job_cancel, :message, 16, 'cordum.agent.v1.JobCancel'
        optional :handshake, :message, 17, 'cordum.agent.v1.Handshake'
      end
      optional :signature, :bytes, 14
    end
  end
end

module Cordum
  module Agent
    module V1
      BusPacket = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.BusPacket').msgclass
    end
  end
end
