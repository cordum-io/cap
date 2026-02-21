# frozen_string_literal: true

require 'spec_helper'

RSpec.describe Cordum::Cap::MockNats do
  it 'tracks published messages' do
    bus = described_class.new
    bus.publish('sys.heartbeat', 'abc')
    bus.publish('sys.job.result', 'def')

    expect(bus.published_count).to eq(2)
    expect(bus.published_to('sys.heartbeat').length).to eq(1)
    expect(bus.published_to('sys.job.result').length).to eq(1)
    expect(bus.published_to('sys.job.submit')).to be_empty
  end

  it 'clears published messages' do
    bus = described_class.new
    bus.publish('sys.heartbeat', 'x')
    expect(bus.published_count).to eq(1)

    bus.clear
    expect(bus.published_count).to eq(0)
  end
end

RSpec.describe Cordum::Cap::RecordingMetrics do
  it 'tracks all metric events' do
    m = described_class.new
    m.on_job_received('j-1', 'test')
    m.on_job_completed('j-1', 100, 'SUCCEEDED')
    m.on_heartbeat_sent('w-1')
    m.on_job_failed('j-2', 'timeout')
    m.on_error('parse', 'invalid proto')

    expect(m.received_jobs.length).to eq(1)
    expect(m.completed_jobs.length).to eq(1)
    expect(m.heartbeats.length).to eq(1)
    expect(m.failed_jobs.length).to eq(1)
    expect(m.errors.length).to eq(1)
    expect(m.errors.first).to eq('parse: invalid proto')
  end

  it 'tracks multiple events of same type' do
    m = described_class.new
    m.on_job_received('j-1', 't')
    m.on_job_received('j-2', 't')
    m.on_job_received('j-3', 't')

    expect(m.received_jobs.length).to eq(3)
  end
end

RSpec.describe Cordum::Cap::Errors do
  it 'creates typed errors with correct codes' do
    err = described_class.job_timeout('timed out')
    expect(err).to be_a(Cordum::Cap::CapError)
    expect(err.code).to eq(Cordum::Cap::ErrorCodes::JOB_TIMEOUT)
    expect(err.message).to eq('timed out')
  end

  it 'creates validation errors with field' do
    err = Cordum::Cap::ValidationError.new('job_id', 'job_id is required')
    expect(err.field).to eq('job_id')
    expect(err.code).to eq(Cordum::Cap::ErrorCodes::JOB_INVALID_INPUT)
  end
end
