# frozen_string_literal: true
# Generated from cordum/agent/v1/alert.proto — DO NOT EDIT

require 'google/protobuf'
require_relative 'job_pb'

Google::Protobuf::DescriptorPool.generated_pool.build do
  add_file('cordum/agent/v1/alert.proto', syntax: :proto3) do
    add_enum 'cordum.agent.v1.AlertSeverity' do
      value :ALERT_SEVERITY_UNSPECIFIED, 0
      value :ALERT_SEVERITY_INFO, 1
      value :ALERT_SEVERITY_WARNING, 2
      value :ALERT_SEVERITY_ERROR, 3
      value :ALERT_SEVERITY_CRITICAL, 4
    end

    add_message 'cordum.agent.v1.SystemAlert' do
      optional :level, :string, 1
      optional :message, :string, 2
      optional :component, :string, 3
      optional :code, :string, 4
      optional :severity, :enum, 5, 'cordum.agent.v1.AlertSeverity'
      optional :error_code_enum, :enum, 6, 'cordum.agent.v1.ErrorCode'
      optional :source_component, :string, 7
      map :details, :string, :string, 8
      optional :trace_id, :string, 9
    end
  end
end

module Cordum
  module Agent
    module V1
      AlertSeverity = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.AlertSeverity').enummodule
      SystemAlert = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.SystemAlert').msgclass
    end
  end
end
