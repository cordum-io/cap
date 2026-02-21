# frozen_string_literal: true

require 'spec_helper'

RSpec.describe Cordum::Cap::ProgressHelper do
  describe '.progress_payload' do
    it 'builds a valid progress packet' do
      bytes = described_class.progress_payload(
        sender_id: 's-1', job_id: 'j-42', step_id: 'step-3',
        percent: 75, message: 'processing'
      )
      expect(bytes).not_to be_empty

      pkt = Cordum::Agent::V1::BusPacket.decode(bytes)
      expect(pkt.sender_id).to eq('s-1')

      p = pkt.job_progress
      expect(p).not_to be_nil
      expect(p.job_id).to eq('j-42')
      expect(p.step_id).to eq('step-3')
      expect(p.percent).to eq(75)
      expect(p.message).to eq('processing')
    end

    it 'handles zero percent' do
      bytes = described_class.progress_payload(
        sender_id: 's', job_id: 'j', step_id: 'st', percent: 0, message: ''
      )
      pkt = Cordum::Agent::V1::BusPacket.decode(bytes)
      expect(pkt.job_progress.percent).to eq(0)
    end

    it 'handles 100 percent' do
      bytes = described_class.progress_payload(
        sender_id: 's', job_id: 'j', step_id: 'st', percent: 100, message: 'done'
      )
      pkt = Cordum::Agent::V1::BusPacket.decode(bytes)
      expect(pkt.job_progress.percent).to eq(100)
      expect(pkt.job_progress.message).to eq('done')
    end

    it 'includes a timestamp' do
      bytes = described_class.progress_payload(
        sender_id: 's', job_id: 'j', step_id: 'st', percent: 50, message: 'half'
      )
      pkt = Cordum::Agent::V1::BusPacket.decode(bytes)
      expect(pkt.created_at).not_to be_nil
      expect(pkt.created_at.seconds).to be > 0
    end
  end

  describe '.cancel_payload' do
    it 'builds a valid cancel packet' do
      bytes = described_class.cancel_payload(
        sender_id: 's-1', job_id: 'j-99', reason: 'timeout', requested_by: 'admin'
      )
      expect(bytes).not_to be_empty

      pkt = Cordum::Agent::V1::BusPacket.decode(bytes)
      c = pkt.job_cancel
      expect(c).not_to be_nil
      expect(c.job_id).to eq('j-99')
      expect(c.reason).to eq('timeout')
      expect(c.requested_by).to eq('admin')
    end

    it 'includes a timestamp' do
      bytes = described_class.cancel_payload(
        sender_id: 's', job_id: 'j', reason: 'r', requested_by: 'u'
      )
      pkt = Cordum::Agent::V1::BusPacket.decode(bytes)
      expect(pkt.created_at).not_to be_nil
    end
  end
end
