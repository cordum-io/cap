# frozen_string_literal: true

require 'spec_helper'

RSpec.describe Cordum::Cap::Validate do
  describe '.job_request!' do
    it 'accepts a valid job request' do
      req = Cordum::Agent::V1::JobRequest.new(job_id: 'j-1', topic: 'test')
      expect { described_class.job_request!(req) }.not_to raise_error
    end

    it 'rejects empty job_id' do
      req = Cordum::Agent::V1::JobRequest.new(job_id: '', topic: 'test')
      expect { described_class.job_request!(req) }
        .to raise_error(Cordum::Cap::ValidationError, /job_id/)
    end

    it 'rejects empty topic' do
      req = Cordum::Agent::V1::JobRequest.new(job_id: 'j-1', topic: '')
      expect { described_class.job_request!(req) }
        .to raise_error(Cordum::Cap::ValidationError, /topic/)
    end
  end

  describe '.job_result!' do
    it 'accepts a valid job result' do
      res = Cordum::Agent::V1::JobResult.new(
        job_id: 'j-1', worker_id: 'w-1', status: :JOB_STATUS_SUCCEEDED
      )
      expect { described_class.job_result!(res) }.not_to raise_error
    end

    it 'rejects empty job_id' do
      res = Cordum::Agent::V1::JobResult.new(
        job_id: '', worker_id: 'w', status: :JOB_STATUS_SUCCEEDED
      )
      expect { described_class.job_result!(res) }
        .to raise_error(Cordum::Cap::ValidationError, /job_id/)
    end

    it 'rejects empty worker_id' do
      res = Cordum::Agent::V1::JobResult.new(
        job_id: 'j', worker_id: '', status: :JOB_STATUS_SUCCEEDED
      )
      expect { described_class.job_result!(res) }
        .to raise_error(Cordum::Cap::ValidationError, /worker_id/)
    end

    it 'rejects unspecified status' do
      res = Cordum::Agent::V1::JobResult.new(
        job_id: 'j', worker_id: 'w', status: :JOB_STATUS_UNSPECIFIED
      )
      expect { described_class.job_result!(res) }
        .to raise_error(Cordum::Cap::ValidationError, /status/)
    end
  end

  describe '.bus_packet!' do
    it 'accepts a valid packet with job_request' do
      req = Cordum::Agent::V1::JobRequest.new(job_id: 'j', topic: 't')
      pkt = Cordum::Agent::V1::BusPacket.new(
        sender_id: 's', protocol_version: 1, job_request: req
      )
      expect { described_class.bus_packet!(pkt) }.not_to raise_error
    end

    it 'rejects empty sender_id' do
      pkt = Cordum::Agent::V1::BusPacket.new(sender_id: '', protocol_version: 1)
      expect { described_class.bus_packet!(pkt) }
        .to raise_error(Cordum::Cap::ValidationError, /sender_id/)
    end

    it 'rejects unsupported protocol version' do
      pkt = Cordum::Agent::V1::BusPacket.new(sender_id: 's', protocol_version: 99)
      expect { described_class.bus_packet!(pkt) }
        .to raise_error(Cordum::Cap::ValidationError, /protocol_version/)
    end
  end

  describe '.heartbeat!' do
    it 'accepts a valid heartbeat' do
      hb = Cordum::Agent::V1::Heartbeat.new(worker_id: 'w-1')
      expect { described_class.heartbeat!(hb) }.not_to raise_error
    end

    it 'rejects empty worker_id' do
      hb = Cordum::Agent::V1::Heartbeat.new(worker_id: '')
      expect { described_class.heartbeat!(hb) }
        .to raise_error(Cordum::Cap::ValidationError, /worker_id/)
    end
  end

  describe '.job_progress!' do
    it 'accepts valid progress' do
      p = Cordum::Agent::V1::JobProgress.new(job_id: 'j', percent: 50)
      expect { described_class.job_progress!(p) }.not_to raise_error
    end

    it 'rejects empty job_id' do
      p = Cordum::Agent::V1::JobProgress.new(job_id: '', percent: 50)
      expect { described_class.job_progress!(p) }
        .to raise_error(Cordum::Cap::ValidationError, /job_id/)
    end
  end

  describe '.job_cancel!' do
    it 'accepts a valid cancel' do
      c = Cordum::Agent::V1::JobCancel.new(job_id: 'j')
      expect { described_class.job_cancel!(c) }.not_to raise_error
    end

    it 'rejects empty job_id' do
      c = Cordum::Agent::V1::JobCancel.new(job_id: '')
      expect { described_class.job_cancel!(c) }
        .to raise_error(Cordum::Cap::ValidationError, /job_id/)
    end
  end
end
