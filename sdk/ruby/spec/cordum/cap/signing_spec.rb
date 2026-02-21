# frozen_string_literal: true

require 'spec_helper'

RSpec.describe Cordum::Cap::Signing do
  let(:keys) { described_class.generate_key_pair }
  let(:private_key) { keys[0] }
  let(:public_key) { keys[1] }

  def make_packet(payload_opts = {})
    req = Cordum::Agent::V1::JobRequest.new(job_id: 'j-1', topic: 'test')
    Cordum::Agent::V1::BusPacket.new(
      { trace_id: 'trace', sender_id: 'sender', protocol_version: 1,
        job_request: req }.merge(payload_opts)
    )
  end

  describe '.sign_packet and .verify_packet_signature' do
    it 'signs and verifies a JobRequest packet' do
      pkt = make_packet
      described_class.sign_packet(pkt, private_key)
      expect(pkt.signature).not_to be_empty
      expect(described_class.verify_packet_signature(pkt, public_key)).to be true
    end

    it 'signs and verifies a Heartbeat packet' do
      hb = Cordum::Agent::V1::Heartbeat.new(worker_id: 'w-1', pool: 'p')
      pkt = Cordum::Agent::V1::BusPacket.new(
        sender_id: 'w-1', protocol_version: 1, heartbeat: hb
      )
      described_class.sign_packet(pkt, private_key)
      expect(described_class.verify_packet_signature(pkt, public_key)).to be true
    end

    it 'signs and verifies a JobResult packet' do
      res = Cordum::Agent::V1::JobResult.new(
        job_id: 'j-1', worker_id: 'w-1', status: :JOB_STATUS_SUCCEEDED
      )
      pkt = Cordum::Agent::V1::BusPacket.new(
        sender_id: 'w-1', protocol_version: 1, job_result: res
      )
      described_class.sign_packet(pkt, private_key)
      expect(described_class.verify_packet_signature(pkt, public_key)).to be true
    end
  end

  describe 'tamper detection' do
    it 'detects tampered trace_id' do
      pkt = make_packet
      described_class.sign_packet(pkt, private_key)
      pkt.trace_id = 'tampered'
      expect(described_class.verify_packet_signature(pkt, public_key)).to be false
    end

    it 'detects tampered sender_id' do
      pkt = make_packet
      described_class.sign_packet(pkt, private_key)
      pkt.sender_id = 'tampered'
      expect(described_class.verify_packet_signature(pkt, public_key)).to be false
    end

    it 'detects tampered protocol_version' do
      pkt = make_packet
      described_class.sign_packet(pkt, private_key)
      pkt.protocol_version = 99
      expect(described_class.verify_packet_signature(pkt, public_key)).to be false
    end

    it 'detects tampered payload' do
      pkt = make_packet
      described_class.sign_packet(pkt, private_key)
      pkt.job_request.job_id = 'tampered'
      expect(described_class.verify_packet_signature(pkt, public_key)).to be false
    end
  end

  describe 'rejection cases' do
    it 'rejects empty signature' do
      pkt = make_packet
      expect(described_class.verify_packet_signature(pkt, public_key)).to be false
    end

    it 'rejects garbage signature' do
      pkt = make_packet
      pkt.signature = 'garbage-bytes'.b
      expect(described_class.verify_packet_signature(pkt, public_key)).to be false
    end

    it 'rejects signature from wrong key' do
      other_keys = described_class.generate_key_pair
      pkt = make_packet
      described_class.sign_packet(pkt, other_keys[0])
      expect(described_class.verify_packet_signature(pkt, public_key)).to be false
    end
  end

  describe '.generate_key_pair' do
    it 'produces a valid key pair' do
      priv, pub = described_class.generate_key_pair
      expect(priv).to be_a(OpenSSL::PKey::EC)
      expect(pub).to be_a(OpenSSL::PKey::EC)
    end
  end

  describe 'signature preserves content' do
    it 'does not alter packet fields after signing' do
      pkt = make_packet
      described_class.sign_packet(pkt, private_key)
      expect(pkt.trace_id).to eq('trace')
      expect(pkt.sender_id).to eq('sender')
      expect(pkt.job_request.job_id).to eq('j-1')
    end
  end

  describe 'double signing' do
    it 'replaces the previous signature' do
      pkt = make_packet
      described_class.sign_packet(pkt, private_key)
      sig1 = pkt.signature.dup
      described_class.sign_packet(pkt, private_key)
      expect(described_class.verify_packet_signature(pkt, public_key)).to be true
      # ECDSA signatures are non-deterministic, so sig may differ
    end
  end
end
