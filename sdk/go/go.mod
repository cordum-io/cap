module github.com/cortex-agent-protocol/sdk/go

go 1.21

require (
	github.com/nats-io/nats.go v1.31.0
	google.golang.org/protobuf v1.33.0
)

// Use locally generated CAP protobuf stubs.
replace github.com/cortex-agent-protocol/go => ../../generated/go
