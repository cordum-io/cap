module github.com/coretexos/cap/sdk/go

go 1.21

require (
	github.com/coretexos/cap v0.0.0-00010101000000-000000000000
	github.com/nats-io/nats.go v1.31.0
	google.golang.org/protobuf v1.33.0
)

require (
	github.com/klauspost/compress v1.17.0 // indirect
	github.com/nats-io/nkeys v0.4.5 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	golang.org/x/net v0.22.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a12d6c2237 // indirect
	google.golang.org/grpc v1.64.0 // indirect
)

// Use locally generated CAP protobuf stubs.
replace github.com/coretexos/cap => ../../generated/go
