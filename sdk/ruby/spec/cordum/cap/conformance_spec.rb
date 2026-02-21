# frozen_string_literal: true

require 'spec_helper'

RSpec.describe 'Conformance' do
  FIXTURES_DIR = File.expand_path('../../../../../../spec/conformance/fixtures', __dir__)

  def load_fixture(name)
    path = File.join(FIXTURES_DIR, name)
    data = File.binread(path)
    Cordum::Agent::V1::BusPacket.decode(data)
  end

  def load_conformance_public_key
    pem = File.read(File.join(FIXTURES_DIR, 'public_key.pem'))
    Cordum::Cap::Signing.load_public_key(pem)
  end

  def verify_fixture_signature(pkt, key)
    expect(pkt.signature).not_to be_empty
    expect(Cordum::Cap::Signing.verify_packet_signature(pkt, key)).to be true
  end

  def assert_timestamp(pkt)
    expect(pkt.created_at).not_to be_nil
    expect(pkt.created_at.seconds).to eq(1_704_164_645)
    expect(pkt.created_at.nanos).to eq(0)
    expect(pkt.protocol_version).to eq(1)
  end

  context 'job_request fixture' do
    it 'decodes and verifies correctly' do
      key = load_conformance_public_key
      pkt = load_fixture('buspacket_job_request.bin')

      verify_fixture_signature(pkt, key)
      assert_timestamp(pkt)

      req = pkt.job_request
      expect(req).not_to be_nil
      expect(req.job_id).to eq('job-req-1')
      expect(req.topic).to eq('job.tools')
      expect(req.priority).to eq(:JOB_PRIORITY_INTERACTIVE)
      expect(req.context_ptr).to eq('redis://ctx:job-req-1')
    end
  end

  context 'job_result fixture' do
    it 'decodes and verifies correctly' do
      key = load_conformance_public_key
      pkt = load_fixture('buspacket_job_result.bin')

      verify_fixture_signature(pkt, key)
      assert_timestamp(pkt)

      res = pkt.job_result
      expect(res).not_to be_nil
      expect(res.job_id).to eq('job-res-1')
      expect(res.worker_id).to eq('worker-1')
      expect(res.status).to eq(:JOB_STATUS_FAILED_RETRYABLE)
      expect(res.error_code).to eq('E_TEMP')
      expect(res.artifact_ptrs.length).to eq(2)
    end
  end

  context 'heartbeat fixture' do
    it 'decodes and verifies correctly' do
      key = load_conformance_public_key
      pkt = load_fixture('buspacket_heartbeat.bin')

      verify_fixture_signature(pkt, key)
      assert_timestamp(pkt)

      hb = pkt.heartbeat
      expect(hb).not_to be_nil
      expect(hb.worker_id).to eq('worker-1')
      expect(hb.pool).to eq('job.tools')
      expect(hb.labels['zone']).to eq('us-east-1a')
    end
  end

  context 'job_progress fixture' do
    it 'decodes and verifies correctly' do
      key = load_conformance_public_key
      pkt = load_fixture('buspacket_job_progress.bin')

      verify_fixture_signature(pkt, key)
      assert_timestamp(pkt)

      progress = pkt.job_progress
      expect(progress).not_to be_nil
      expect(progress.job_id).to eq('job-prog-1')
      expect(progress.percent).to eq(50)
      expect(progress.status).to eq(:JOB_STATUS_RUNNING)
    end
  end

  context 'job_cancel fixture' do
    it 'decodes and verifies correctly' do
      key = load_conformance_public_key
      pkt = load_fixture('buspacket_job_cancel.bin')

      verify_fixture_signature(pkt, key)
      assert_timestamp(pkt)

      cancel = pkt.job_cancel
      expect(cancel).not_to be_nil
      expect(cancel.job_id).to eq('job-cancel-1')
      expect(cancel.requested_by).to eq('user-7')
    end
  end

  context 'alert fixture' do
    it 'decodes and verifies correctly' do
      key = load_conformance_public_key
      pkt = load_fixture('buspacket_alert.bin')

      verify_fixture_signature(pkt, key)
      assert_timestamp(pkt)

      alert = pkt.alert
      expect(alert).not_to be_nil
      expect(alert.level).to eq('WARN')
      expect(alert.component).to eq('scheduler')
    end
  end

  context 'handshake fixture' do
    it 'decodes and verifies correctly' do
      key = load_conformance_public_key
      pkt = load_fixture('buspacket_handshake.bin')

      verify_fixture_signature(pkt, key)
      assert_timestamp(pkt)

      hs = pkt.handshake
      expect(hs).not_to be_nil
      expect(hs.component_id).to eq('worker-1')
      expect(hs.role).to eq(:COMPONENT_ROLE_WORKER)
      expect(hs.sdk_version).to eq('2.0.19')
    end
  end

  context 'alert_enhanced fixture' do
    it 'decodes and verifies correctly' do
      key = load_conformance_public_key
      pkt = load_fixture('buspacket_alert_enhanced.bin')

      verify_fixture_signature(pkt, key)
      assert_timestamp(pkt)

      alert = pkt.alert
      expect(alert).not_to be_nil
      expect(alert.level).to eq('CRITICAL')
      expect(alert.component).to eq('scheduler')
      expect(alert.code).to eq('SIGNATURE_INVALID')
      expect(alert.severity).to eq(:ALERT_SEVERITY_CRITICAL)
      expect(alert.source_component).to eq('scheduler-1')
      expect(alert.details['sender']).to eq('worker-bad')
      expect(alert.details['subject']).to eq('sys.job.result')
      expect(alert.trace_id).to eq('trace-offending-packet')
    end
  end

  context 'all fixtures signature verification' do
    it 'verifies all fixture signatures using unsigned bytes' do
      key = load_conformance_public_key
      fixtures = %w[
        buspacket_job_request.bin
        buspacket_job_result.bin
        buspacket_heartbeat.bin
        buspacket_job_progress.bin
        buspacket_job_cancel.bin
        buspacket_alert.bin
        buspacket_handshake.bin
        buspacket_alert_enhanced.bin
      ]

      fixtures.each do |fixture|
        pkt = load_fixture(fixture)
        expect(pkt.signature).not_to be_empty, "#{fixture} has no signature"

        unsigned_bytes = Cordum::Cap::Codec.marshal_unsigned_for_signature(pkt)
        digest = OpenSSL::Digest::SHA256.new
        verified = key.verify(digest, pkt.signature, unsigned_bytes)
        expect(verified).to be(true), "#{fixture} signature verification failed"
      end
    end
  end
end
