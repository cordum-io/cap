# frozen_string_literal: true

require 'spec_helper'

RSpec.describe Cordum::Cap::HeartbeatHelper do
  describe '.heartbeat_payload' do
    it 'builds a valid heartbeat packet' do
      bytes = described_class.heartbeat_payload(
        worker_id: 'w-1', pool: 'pool-a',
        active_jobs: 3, max_parallel: 8, cpu_load: 42.5
      )
      expect(bytes).not_to be_empty

      pkt = Cordum::Agent::V1::BusPacket.decode(bytes)
      expect(pkt.sender_id).to eq('w-1')
      expect(pkt.protocol_version).to eq(1)

      hb = pkt.heartbeat
      expect(hb).not_to be_nil
      expect(hb.worker_id).to eq('w-1')
      expect(hb.pool).to eq('pool-a')
      expect(hb.active_jobs).to eq(3)
      expect(hb.max_parallel_jobs).to eq(8)
      expect(hb.cpu_load).to be_within(0.01).of(42.5)
    end

    it 'has a timestamp' do
      bytes = described_class.heartbeat_payload(
        worker_id: 'w', pool: 'p', active_jobs: 0, max_parallel: 1, cpu_load: 0.0
      )
      pkt = Cordum::Agent::V1::BusPacket.decode(bytes)
      expect(pkt.created_at).not_to be_nil
      expect(pkt.created_at.seconds).to be > 0
    end

    it 'handles zero active jobs' do
      bytes = described_class.heartbeat_payload(
        worker_id: 'w', pool: 'p', active_jobs: 0, max_parallel: 4, cpu_load: 0.0
      )
      pkt = Cordum::Agent::V1::BusPacket.decode(bytes)
      expect(pkt.heartbeat.active_jobs).to eq(0)
    end
  end

  describe '.heartbeat_payload_with_memory' do
    it 'includes memory_load' do
      bytes = described_class.heartbeat_payload_with_memory(
        worker_id: 'w-1', pool: 'p', active_jobs: 1, max_parallel: 4,
        cpu_load: 10.0, memory_load: 55.0
      )
      pkt = Cordum::Agent::V1::BusPacket.decode(bytes)
      expect(pkt.heartbeat.memory_load).to be_within(0.01).of(55.0)
      expect(pkt.heartbeat.progress_pct).to eq(0)
    end
  end

  describe '.heartbeat_payload_with_progress' do
    it 'includes all fields' do
      bytes = described_class.heartbeat_payload_with_progress(
        worker_id: 'w-2', pool: 'gpu', active_jobs: 5, max_parallel: 10,
        cpu_load: 80.0, memory_load: 60.0, progress_pct: 75, last_memo: 'step-3'
      )
      pkt = Cordum::Agent::V1::BusPacket.decode(bytes)
      hb = pkt.heartbeat

      expect(hb.worker_id).to eq('w-2')
      expect(hb.pool).to eq('gpu')
      expect(hb.active_jobs).to eq(5)
      expect(hb.progress_pct).to eq(75)
      expect(hb.last_memo).to eq('step-3')
      expect(hb.cpu_load).to be_within(0.01).of(80.0)
      expect(hb.memory_load).to be_within(0.01).of(60.0)
    end
  end
end
