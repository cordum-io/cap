# frozen_string_literal: true
# Generated from cordum/agent/v1/job.proto — DO NOT EDIT

require 'google/protobuf'

Google::Protobuf::DescriptorPool.generated_pool.build do
  add_file('cordum/agent/v1/job.proto', syntax: :proto3) do
    add_enum 'cordum.agent.v1.JobPriority' do
      value :JOB_PRIORITY_UNSPECIFIED, 0
      value :JOB_PRIORITY_INTERACTIVE, 1
      value :JOB_PRIORITY_BATCH, 2
      value :JOB_PRIORITY_CRITICAL, 3
    end

    add_enum 'cordum.agent.v1.JobStatus' do
      value :JOB_STATUS_UNSPECIFIED, 0
      value :JOB_STATUS_PENDING, 1
      value :JOB_STATUS_SCHEDULED, 2
      value :JOB_STATUS_DISPATCHED, 3
      value :JOB_STATUS_RUNNING, 4
      value :JOB_STATUS_SUCCEEDED, 5
      value :JOB_STATUS_FAILED, 6
      value :JOB_STATUS_CANCELLED, 7
      value :JOB_STATUS_DENIED, 8
      value :JOB_STATUS_TIMEOUT, 9
      value :JOB_STATUS_FAILED_RETRYABLE, 10
      value :JOB_STATUS_FAILED_FATAL, 11
    end

    add_enum 'cordum.agent.v1.ActorType' do
      value :ACTOR_TYPE_UNSPECIFIED, 0
      value :ACTOR_TYPE_HUMAN, 1
      value :ACTOR_TYPE_SERVICE, 2
    end

    add_enum 'cordum.agent.v1.ErrorCode' do
      value :ERROR_CODE_UNSPECIFIED, 0
      value :ERROR_CODE_PROTOCOL_VERSION_MISMATCH, 100
      value :ERROR_CODE_PROTOCOL_MALFORMED_PACKET, 101
      value :ERROR_CODE_PROTOCOL_UNKNOWN_PAYLOAD, 102
      value :ERROR_CODE_PROTOCOL_SIGNATURE_INVALID, 103
      value :ERROR_CODE_PROTOCOL_SIGNATURE_MISSING, 104
      value :ERROR_CODE_JOB_TIMEOUT, 200
      value :ERROR_CODE_JOB_RESOURCE_EXHAUSTED, 201
      value :ERROR_CODE_JOB_PERMISSION_DENIED, 202
      value :ERROR_CODE_JOB_INVALID_INPUT, 203
      value :ERROR_CODE_JOB_NOT_FOUND, 204
      value :ERROR_CODE_JOB_DUPLICATE, 205
      value :ERROR_CODE_JOB_WORKER_UNAVAILABLE, 206
      value :ERROR_CODE_SAFETY_DENIED, 300
      value :ERROR_CODE_SAFETY_POLICY_VIOLATION, 301
      value :ERROR_CODE_SAFETY_RISK_TAG_BLOCKED, 302
      value :ERROR_CODE_TRANSPORT_PUBLISH_FAILED, 400
      value :ERROR_CODE_TRANSPORT_SUBSCRIBE_FAILED, 401
      value :ERROR_CODE_TRANSPORT_CONNECTION_LOST, 402
    end

    add_message 'cordum.agent.v1.ContextHints' do
      optional :max_input_tokens, :int32, 1
      optional :allow_summarization, :bool, 2
      optional :allow_retrieval, :bool, 3
      repeated :tags, :string, 4
    end

    add_message 'cordum.agent.v1.Budget' do
      optional :max_input_tokens, :int64, 1
      optional :max_output_tokens, :int64, 2
      optional :max_total_tokens, :int64, 3
      optional :deadline_ms, :int64, 4
    end

    add_message 'cordum.agent.v1.JobMetadata' do
      optional :tenant_id, :string, 1
      optional :actor_id, :string, 2
      optional :actor_type, :enum, 3, 'cordum.agent.v1.ActorType'
      optional :idempotency_key, :string, 4
      optional :capability, :string, 5
      repeated :risk_tags, :string, 6
      repeated :requires, :string, 7
      optional :pack_id, :string, 8
      map :labels, :string, :string, 9
    end

    add_message 'cordum.agent.v1.Compensation' do
      optional :topic, :string, 1
      optional :context_ptr, :string, 2
      optional :priority, :enum, 3, 'cordum.agent.v1.JobPriority'
      optional :adapter_id, :string, 4
      map :env, :string, :string, 5
      optional :memory_id, :string, 6
      optional :context_hints, :message, 7, 'cordum.agent.v1.ContextHints'
      optional :budget, :message, 8, 'cordum.agent.v1.Budget'
      optional :tenant_id, :string, 9
      optional :principal_id, :string, 10
      map :labels, :string, :string, 11
      optional :meta, :message, 12, 'cordum.agent.v1.JobMetadata'
    end

    add_message 'cordum.agent.v1.JobRequest' do
      optional :job_id, :string, 1
      optional :topic, :string, 2
      optional :priority, :enum, 3, 'cordum.agent.v1.JobPriority'
      optional :context_ptr, :string, 4
      optional :adapter_id, :string, 5
      map :env, :string, :string, 6
      optional :parent_job_id, :string, 7
      optional :workflow_id, :string, 8
      optional :step_index, :int32, 9
      optional :memory_id, :string, 10
      optional :context_hints, :message, 11, 'cordum.agent.v1.ContextHints'
      optional :budget, :message, 12, 'cordum.agent.v1.Budget'
      optional :tenant_id, :string, 13
      optional :principal_id, :string, 14
      map :labels, :string, :string, 15
      optional :meta, :message, 16, 'cordum.agent.v1.JobMetadata'
      optional :compensation, :message, 17, 'cordum.agent.v1.Compensation'
    end

    add_message 'cordum.agent.v1.JobResult' do
      optional :job_id, :string, 1
      optional :status, :enum, 2, 'cordum.agent.v1.JobStatus'
      optional :result_ptr, :string, 3
      optional :worker_id, :string, 4
      optional :execution_ms, :int64, 5
      optional :error_code, :string, 6
      optional :error_message, :string, 7
      repeated :artifact_ptrs, :string, 8
      optional :error_code_enum, :enum, 9, 'cordum.agent.v1.ErrorCode'
    end

    add_message 'cordum.agent.v1.JobProgress' do
      optional :job_id, :string, 1
      optional :step_id, :string, 2
      optional :percent, :int32, 3
      optional :message, :string, 4
      optional :result_ptr, :string, 5
      repeated :artifact_ptrs, :string, 6
      optional :status, :enum, 7, 'cordum.agent.v1.JobStatus'
    end

    add_message 'cordum.agent.v1.JobCancel' do
      optional :job_id, :string, 1
      optional :reason, :string, 2
      optional :requested_by, :string, 3
    end
  end
end

module Cordum
  module Agent
    module V1
      JobPriority = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.JobPriority').enummodule
      JobStatus = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.JobStatus').enummodule
      ActorType = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.ActorType').enummodule
      ErrorCode = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.ErrorCode').enummodule
      ContextHints = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.ContextHints').msgclass
      Budget = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.Budget').msgclass
      JobMetadata = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.JobMetadata').msgclass
      Compensation = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.Compensation').msgclass
      JobRequest = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.JobRequest').msgclass
      JobResult = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.JobResult').msgclass
      JobProgress = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.JobProgress').msgclass
      JobCancel = ::Google::Protobuf::DescriptorPool.generated_pool.lookup('cordum.agent.v1.JobCancel').msgclass
    end
  end
end
