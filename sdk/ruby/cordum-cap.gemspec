# frozen_string_literal: true

Gem::Specification.new do |spec|
  spec.name          = 'cordum-cap'
  spec.version       = '2.5.3'
  spec.authors       = ['Cordum']
  spec.email         = ['dev@cordum.io']
  spec.summary       = 'Ruby SDK for the Cordum Agent Protocol (CAP)'
  spec.description   = 'Client and worker SDK for building agents on the Cordum Agent Protocol bus.'
  spec.homepage      = 'https://github.com/cordum-io/cap'
  spec.license       = 'Apache-2.0'
  spec.required_ruby_version = '>= 3.1'

  spec.files         = Dir['lib/**/*.rb', 'proto/**/*.rb', 'LICENSE', 'README.md']
  spec.require_paths = ['lib', 'proto']

  spec.add_dependency 'google-protobuf', '~> 4.0'
  spec.add_dependency 'nats-pure', '~> 2.0'

  spec.add_development_dependency 'rspec', '~> 3.0'
end
