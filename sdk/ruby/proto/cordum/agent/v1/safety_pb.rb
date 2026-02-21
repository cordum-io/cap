# frozen_string_literal: true
# Generated from cordum/agent/v1/safety.proto — DO NOT EDIT

require 'google/protobuf'
require_relative 'job_pb'

Google::Protobuf::DescriptorPool.generated_pool.build do
  add_file('cordum/agent/v1/safety.proto', syntax: :proto3) do
    add_enum 'cordum.agent.v1.DecisionType' do
      value :DECISION_TYPE_UNSPECIFIED, 0
      value :DECISION_TYPE_ALLOW, 1
      value :DECISION_TYPE_DENY, 2
      value :DECISION_TYPE_REQUIRE_HUMAN, 3
      value :DECISION_TYPE_THROTTLE, 4
      value :DECISION_TYPE_ALLOW_WITH_CONSTRAINTS, 5
    end

    add_message 'cordum.agent.v1.PolicyCheckRequest' do
      optional :job_id, :string, 1
      optional :topic, :string, 2
      optional :tenant, :string, 3
      optional :priority, :enum, 4, 'cordum.agent.v1.JobPriority'
      optional :estimated_cost, :double, 5
      optional :budget, :message, 6, 'cordum.agent.v1.Budget'
      optional :principal_id, :string, 7
      map :labels, :string, :string, 8
      optional :memory_id, :string, 9
      optional :effective_config, :bytes, 10
      optional :meta, :message, 11, 'cordum.agent.v1.JobMetadata'
    end

    add_message 'cordum.agent.v1.BudgetConstraints' do
      optional :max_runtime_ms, :int64, 1
      optional :max_retries, :int32, 2
      optional :max_artifact_bytes, :int64, 3
      optional :max_concurrent_jobs, :int32, 4
    end

    add_message 'cordum.agent.v1.SandboxProfile' do
      optional :isolated, :bool, 1
      repeated :network_allowlist, :string, 2
      repeated :fs_read_only, :string, 3
      repeated :fs_read_write, :string, 4
    end

    add_message 'cordum.agent.v1.ToolchainConstraints' do
      repeated :allowed_tools, :string, 1
      repeated :allowed_commands, :string, 2
    end

    add_message 'cordum.agent.v1.DiffConstraints' do
      optional :max_files, :int32, 1
      optional :max_lines, :int32, 2
      repeated :deny_path_globs, :string, 3
    end

    add_message 'cordum.agent.v1.PolicyConstraints' do
      optional :budgets, :message, 1, 'cordum.agent.v1.BudgetConstraints'
      optional :sandbox, :message, 2, 'cordum.agent.v1.SandboxProfile'
      optional :toolchain, :message, 3, 'cordum.agent.v1.ToolchainConstraints'
      optional :diff, :message, 4, 'cordum.agent.v1.DiffConstraints'
      optional :redaction_level, :string, 5
    end

    add_message 'cordum.agent.v1.PolicyRemediation' do
      optional :id, :string, 1
      optional :title, :string, 2
      optional :summary, :string, 3
      optional :replacement_topic, :string, 4
      optional :replacement_capability, :string, 5
      map :add_labels, :string, :string, 6
      repeated :remove_labels, :string, 7
    end

    add_message 'cordum.agent.v1.PolicyCheckResponse' do
      optional :decision, :enum, 1, 'cordum.agent.v1.DecisionType'
      optional :reason, :string, 2
      optional :redacted_context_ptr, :string, 3
      optional :policy_snapshot, :string, 4
      optional :rule_id, :string, 5
      optional :constraints, :message, 6, 'cordum.agent.v1.PolicyConstraints'
      optional :approval_required, :bool, 7
      optional :approval_ref, :string, 8
      repeated :remediations, :message, 9, 'cordum.agent.v1.PolicyRemediation'
    end
  end
end

module Cordum
  module Agent
    module V1
      DecisionType = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.DecisionType').enummodule
      PolicyCheckRequest = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.PolicyCheckRequest').msgclass
      BudgetConstraints = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.BudgetConstraints').msgclass
      SandboxProfile = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.SandboxProfile').msgclass
      ToolchainConstraints = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.ToolchainConstraints').msgclass
      DiffConstraints = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.DiffConstraints').msgclass
      PolicyConstraints = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.PolicyConstraints').msgclass
      PolicyRemediation = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.PolicyRemediation').msgclass
      PolicyCheckResponse = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.PolicyCheckResponse').msgclass
    end
  end
end
