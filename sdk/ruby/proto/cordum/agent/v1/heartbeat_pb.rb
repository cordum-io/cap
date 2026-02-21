# frozen_string_literal: true
# Generated from cordum/agent/v1/heartbeat.proto — DO NOT EDIT

require 'google/protobuf'

Google::Protobuf::DescriptorPool.generated_pool.build do
  add_file('cordum/agent/v1/heartbeat.proto', syntax: :proto3) do
    add_message 'cordum.agent.v1.Heartbeat' do
      optional :worker_id, :string, 1
      optional :region, :string, 2
      optional :type, :string, 3
      optional :cpu_load, :float, 4
      optional :gpu_utilization, :float, 5
      optional :active_jobs, :int32, 6
      repeated :capabilities, :string, 7
      optional :pool, :string, 11
      optional :max_parallel_jobs, :int32, 12
      map :labels, :string, :string, 13
      optional :memory_load, :float, 14
      optional :progress_pct, :int32, 15
      optional :last_memo, :string, 16
    end
  end
end

module Cordum
  module Agent
    module V1
      Heartbeat = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.Heartbeat').msgclass
    end
  end
end
