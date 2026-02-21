# frozen_string_literal: true
# Generated from cordum/agent/v1/handshake.proto — DO NOT EDIT

require 'google/protobuf'

Google::Protobuf::DescriptorPool.generated_pool.build do
  add_file('cordum/agent/v1/handshake.proto', syntax: :proto3) do
    add_enum 'cordum.agent.v1.ComponentRole' do
      value :COMPONENT_ROLE_UNSPECIFIED, 0
      value :COMPONENT_ROLE_GATEWAY, 1
      value :COMPONENT_ROLE_SCHEDULER, 2
      value :COMPONENT_ROLE_WORKER, 3
      value :COMPONENT_ROLE_ORCHESTRATOR, 4
      value :COMPONENT_ROLE_CONTROLLER, 5
    end

    add_message 'cordum.agent.v1.Handshake' do
      optional :component_id, :string, 1
      optional :role, :enum, 2, 'cordum.agent.v1.ComponentRole'
      repeated :supported_versions, :int32, 3
      map :capabilities, :string, :bool, 4
      optional :sdk_version, :string, 5
    end
  end
end

module Cordum
  module Agent
    module V1
      ComponentRole = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.ComponentRole').enummodule
      Handshake = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.Handshake').msgclass
    end
  end
end
