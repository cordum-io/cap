#!/usr/bin/env bash
# Runs INSIDE the pinned codegen container; driven by mutation_check.sh.
#
# Works on a throwaway copy of the read-only source mount, so the real
# repository is never mutated: change one proto, regenerate, and require every
# language's output for that proto to change. A generator that copied the
# tracked files, or a language silently emitting nothing, passes `--check` but
# fails here.
set -euo pipefail

work=/work
cp -r /src "${work}"
cd "${work}"

# One output per generated language surface, all derived from policy.proto.
# policy.proto declares a service, so the gRPC stub modules are in scope too;
# policy.proto is also a Node aggregate root, hence node/cap_pb.*.
outputs="
cordum/agent/v1/policy.pb.go
cordum/agent/v1/policy_grpc.pb.go
cpp/cordum/agent/v1/policy.pb.cc
cpp/cordum/agent/v1/policy.pb.h
node/cordum/agent/v1/policy_pb.js
node/cordum/agent/v1/policy_pb.d.ts
node/cap_pb.js
node/cap_pb.d.ts
python/cordum/agent/v1/policy_pb2.py
python/cordum/agent/v1/policy_pb2_grpc.py
sdk/python/cap/pb/cordum/agent/v1/policy_pb2.py
sdk/python/cap/pb/cordum/agent/v1/policy_pb2_grpc.py
sdk/ruby/proto/cordum/agent/v1/policy_pb.rb
"

echo ">> baseline generation"
python3 tools/proto_codegen.py --write --offline >/dev/null
for f in ${outputs}; do install -D "${work}/${f}" "/baseline/${f}"; done

echo ">> mutating proto/cordum/agent/v1/policy.proto"
python3 - "${work}/proto/cordum/agent/v1/policy.proto" <<'PY'
import re, sys

path = sys.argv[1]
text = open(path, encoding="utf-8").read()
# buf lint requires distinct request and response types per rpc.
text += (
    "\nmessage CodegenMutationProbeRequest {\n  string probe_field = 1;\n}\n"
    "\nmessage CodegenMutationProbeResponse {\n  string probe_field = 1;\n}\n"
)
text, count = re.subn(
    r"(service\s+\w+\s*\{)",
    r"\1\n  rpc CodegenMutationProbe(CodegenMutationProbeRequest) returns (CodegenMutationProbeResponse);",
    text, count=1)
if count != 1:
    raise SystemExit("policy.proto no longer declares a service; the probe would not reach the gRPC stubs")
open(path, "w", encoding="utf-8").write(text)
PY

echo ">> regenerating from the mutated proto"
python3 tools/proto_codegen.py --write --offline >/dev/null

status=0
total=0
for f in ${outputs}; do
  total=$((total + 1))
  if cmp -s "/baseline/${f}" "${work}/${f}"; then
    echo "UNCHANGED ${f} -- this output does not derive from policy.proto"
    status=1
  else
    echo "changed   ${f}"
  fi
done

if [ ${status} -ne 0 ]; then
  echo "MUTATION CHECK FAILED: at least one language ignored the proto change" >&2
  exit 1
fi
echo "MUTATION CHECK PASSED: all ${total} declared outputs changed"
