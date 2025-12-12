# CAP Node JS Protobuf Stubs

CommonJS protobuf stubs generated from `proto/coretex/agent/v1/*.proto`:

```bash
./tools/make_protos.sh   # writes to /node by default
```
Artifacts:
- `cap_pb.js` / `cap_pb.d.ts` — bundled static modules for all messages.

Use with `protobufjs` or plain `google-protobuf` runtime:

```js
const { BusPacket } = require('./coretex/agent/v1/buspacket_pb');
const pkt = new BusPacket();
pkt.setTraceId('trace-1');
```

Notes:
- Install `protoc-gen-js` (e.g., `npm install -g google-protobuf`) to enable JS generation; the helper script will skip if the plugin is absent.
- TypeScript typings will emit when `protoc-gen-ts` is on `PATH`.
- Regenerate after proto changes to keep the JS modules current.
