# frozen_string_literal: true

require 'spec_helper'

RSpec.describe Cordum::Cap::Worker do
  describe 'TestHelper.submit_and_wait' do
    it 'returns a result from the handler' do
      handler = lambda do |ctx, req|
        Cordum::Agent::V1::JobResult.new(
          job_id: req.job_id, worker_id: 'test-w',
          status: :JOB_STATUS_SUCCEEDED
        )
      end

      req = Cordum::Agent::V1::JobRequest.new(job_id: 'j-1', topic: 'test')
      result = Cordum::Cap::TestHelper.submit_and_wait(handler, req)

      expect(result.job_id).to eq('j-1')
      expect(result.status).to eq(:JOB_STATUS_SUCCEEDED)
    end

    it 'populates context with test values' do
      captured_ctx = nil
      handler = lambda do |ctx, req|
        captured_ctx = ctx
        Cordum::Agent::V1::JobResult.new(
          job_id: req.job_id, worker_id: 'w',
          status: :JOB_STATUS_SUCCEEDED
        )
      end

      req = Cordum::Agent::V1::JobRequest.new(job_id: 'j', topic: 't')
      Cordum::Cap::TestHelper.submit_and_wait(handler, req)

      expect(captured_ctx.trace_id).to eq('test-trace')
      expect(captured_ctx.sender_id).to eq('test-sender')
    end

    it 'propagates handler exceptions' do
      handler = lambda do |_ctx, _req|
        raise Cordum::Cap::Errors.job_timeout('timed out')
      end

      req = Cordum::Agent::V1::JobRequest.new(job_id: 'j', topic: 't')
      expect { Cordum::Cap::TestHelper.submit_and_wait(handler, req) }
        .to raise_error(Cordum::Cap::CapError)
    end
  end

  describe Cordum::Cap::MiddlewareChain do
    it 'applies middlewares in FIFO order' do
      order = []

      make_mw = lambda do |id|
        lambda do |handler|
          lambda do |ctx, req|
            order << id
            handler.call(ctx, req)
          end
        end
      end

      base_handler = lambda do |_ctx, req|
        Cordum::Agent::V1::JobResult.new(
          job_id: req.job_id, worker_id: 'w',
          status: :JOB_STATUS_SUCCEEDED
        )
      end

      wrapped = described_class.apply(base_handler, [make_mw.call(1), make_mw.call(2), make_mw.call(3)])
      ctx = Cordum::Cap::Context.new(trace_id: 't', sender_id: 's')
      req = Cordum::Agent::V1::JobRequest.new(job_id: 'j', topic: 't')
      wrapped.call(ctx, req)

      expect(order).to eq([1, 2, 3])
    end

    it 'returns base handler when no middlewares' do
      base_handler = lambda do |_ctx, req|
        Cordum::Agent::V1::JobResult.new(
          job_id: req.job_id, worker_id: 'w',
          status: :JOB_STATUS_SUCCEEDED
        )
      end

      wrapped = described_class.apply(base_handler, [])
      ctx = Cordum::Cap::Context.new(trace_id: 't', sender_id: 's')
      req = Cordum::Agent::V1::JobRequest.new(job_id: 'j-1', topic: 't')
      result = wrapped.call(ctx, req)

      expect(result.job_id).to eq('j-1')
    end
  end
end
