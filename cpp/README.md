# CAP C++ Protobuf Stubs

Pre-generated C++ sources/headers for CAP messages (`cpp/coretex/agent/v1/*.pb.{h,cc}`).
Built via:

```bash
./tools/make_protos.sh   # generates into /cpp by default
```

Usage:
- Add `cpp/` to your include paths (`-Icpp`) and link against `libprotobuf`.
- Or point `sdk/cpp` CMake at `PROTO_ROOT=../../proto` to regenerate during your build.

Regenerate after any proto changes to keep the wire layout in sync.
