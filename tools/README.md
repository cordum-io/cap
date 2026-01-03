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
- Go stubs under `/coretex/agent/v1`
- C++ stubs under `/cpp`
- Node (JS) stubs under `/node` (CommonJS, binary wire)
- Python stubs under `/python` (set `CAP_RUN_PY=1`)

Env toggles:
- `CAP_OUT_GO`, `CAP_OUT_CPP`, `CAP_OUT_JS`, `CAP_OUT_PY` to override output dirs.
- `CAP_RUN_PY=1` to enable Python stubs.
- `CAP_RUN_CPP=0` or `CAP_RUN_JS=0` to skip C++/Node.
- `protoc-gen-ts` on PATH will emit TS typings alongside JS.

Adjust flags/paths as needed for other languages or build systems.
