# CAP Node JS Protobuf Stubs

CommonJS protobuf stubs generated from `proto/cordum/agent/v1/*.proto`:

```bash
./tools/make_protos.sh   # writes to /node by default
```
Artifacts:
- `cap_pb.js` / `cap_pb.d.ts` — bundled static modules for all messages.

Use with `protobufjs` or plain `google-protobuf` runtime:

```js
const { BusPacket } = require('./cordum/agent/v1/buspacket_pb');
const pkt = new BusPacket();
pkt.setTraceId('trace-1');
```

Notes:
- Ensure `protoc` is on `PATH` to emit JS stubs; the helper script will skip if no generator is available.
- TypeScript typings will emit when `protoc-gen-ts` is on `PATH`, or `pbts` can emit `cap_pb.d.ts` from the bundle.
- Regenerate after proto changes to keep the JS modules current.
