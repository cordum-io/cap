# CAP Python Protobuf Stubs

Generated protobuf+gRPC Python modules for CAP. Produced by running:

```bash
CAP_RUN_PY=1 ./tools/make_protos.sh
```

Contents mirror `proto/coretex/agent/v1/*.proto` under `python/coretex/agent/v1/`. You can:
- add `python/` to `PYTHONPATH` and import `coretex.agent.v1.*`, or
- copy the files into `sdk/python/cap/pb` to vend them with the SDK package.

Regenerate after any proto change to keep wire compatibility. The stubs were built with `protoc` 3.21.12.
