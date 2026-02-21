# frozen_string_literal: true

$LOAD_PATH.unshift File.expand_path('../lib', __dir__)
$LOAD_PATH.unshift File.expand_path('../proto', __dir__)

require 'cordum/agent/v1/buspacket_pb'
require 'cordum/cap'

RSpec.configure do |config|
  config.expect_with :rspec do |expectations|
    expectations.include_chain_clauses_in_custom_matcher_descriptions = true
  end

  config.mock_with :rspec do |mocks|
    mocks.verify_partial_doubles = true
  end

  config.shared_context_metadata_behavior = :apply_to_host_groups
end
