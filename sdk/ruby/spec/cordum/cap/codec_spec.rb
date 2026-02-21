# frozen_string_literal: true

require 'spec_helper'

RSpec.describe Cordum::Cap::Codec do
  describe '.marshal_deterministic' do
    it 'produces identical bytes across 100 iterations for JobRequest' do
      req = Cordum::Agent::V1::JobRequest.new(job_id: 'j-1', topic: 'test.topic')
      pkt = Cordum::Agent::V1::BusPacket.new(
        trace_id: 'trace-1', sender_id: 'sender-1',
        protocol_version: 1, job_request: req
      )

      first = described_class.marshal_deterministic(pkt)
      100.times do
        expect(described_class.marshal_deterministic(pkt)).to eq(first)
      end
    end

    it 'produces identical bytes across 100 iterations for Heartbeat' do
      hb = Cordum::Agent::V1::Heartbeat.new(worker_id: 'w-1', pool: 'cpu', active_jobs: 3)
      pkt = Cordum::Agent::V1::BusPacket.new(
        sender_id: 'w-1', protocol_version: 1, heartbeat: hb
      )

      first = described_class.marshal_deterministic(pkt)
      100.times do
        expect(described_class.marshal_deterministic(pkt)).to eq(first)
      end
    end

    it 'round-trips a JobRequest packet' do
      req = Cordum::Agent::V1::JobRequest.new(
        job_id: 'j-rt', topic: 'rt.topic',
        priority: :JOB_PRIORITY_INTERACTIVE, context_ptr: 'redis://ctx'
      )
      pkt = Cordum::Agent::V1::BusPacket.new(
        trace_id: 't', sender_id: 's', protocol_version: 1, job_request: req
      )

      bytes = described_class.marshal_deterministic(pkt)
      decoded = Cordum::Agent::V1::BusPacket.decode(bytes)
      expect(decoded.job_request.job_id).to eq('j-rt')
      expect(decoded.job_request.topic).to eq('rt.topic')
    end

    it 'round-trips a Heartbeat packet' do
      hb = Cordum::Agent::V1::Heartbeat.new(
        worker_id: 'w-rt', pool: 'gpu', active_jobs: 5, cpu_load: 42.5
      )
      pkt = Cordum::Agent::V1::BusPacket.new(
        sender_id: 'w-rt', protocol_version: 1, heartbeat: hb
      )

      bytes = described_class.marshal_deterministic(pkt)
      decoded = Cordum::Agent::V1::BusPacket.decode(bytes)
      expect(decoded.heartbeat.worker_id).to eq('w-rt')
      expect(decoded.heartbeat.cpu_load).to be_within(0.01).of(42.5)
    end

    it 'produces different bytes for different packets' do
      pkt1 = Cordum::Agent::V1::BusPacket.new(sender_id: 'a', protocol_version: 1)
      pkt2 = Cordum::Agent::V1::BusPacket.new(sender_id: 'b', protocol_version: 1)

      expect(described_class.marshal_deterministic(pkt1))
        .not_to eq(described_class.marshal_deterministic(pkt2))
    end

    it 'serializes an empty packet' do
      pkt = Cordum::Agent::V1::BusPacket.new
      bytes = described_class.marshal_deterministic(pkt)
      expect(bytes).to be_a(String)
    end
  end

  describe '.marshal_unsigned_for_signature' do
    it 'clears the signature field' do
      pkt = Cordum::Agent::V1::BusPacket.new(
        sender_id: 's', protocol_version: 1, signature: 'some-sig'.b
      )

      unsigned_bytes = described_class.marshal_unsigned_for_signature(pkt)
      decoded = Cordum::Agent::V1::BusPacket.decode(unsigned_bytes)
      expect(decoded.signature).to eq(''.b)
    end

    it 'does not modify the original packet' do
      pkt = Cordum::Agent::V1::BusPacket.new(
        sender_id: 's', protocol_version: 1, signature: 'keep-me'.b
      )

      described_class.marshal_unsigned_for_signature(pkt)
      expect(pkt.signature).to eq('keep-me'.b)
    end
  end
end
