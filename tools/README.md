# Tools

These helpers are optional. CAP is language-agnostic; use these scripts only if you want generated stubs for local development.

## make_protos.sh
Prereqs:
- `protoc`
- For Go: `protoc-gen-go`, `protoc-gen-go-grpc`
- For Python: `python -m grpc_tools.protoc`

Run:
```bash
./tools/make_protos.sh
```
Outputs:
- Go stubs under `generated/go`
- Python stubs under `generated/python`

Adjust flags/paths as needed for other languages or build systems.
