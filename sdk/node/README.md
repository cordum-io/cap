# CAP Node/TypeScript SDK

Node/TS SDK with NATS helpers. Uses `protobufjs` to load CAP proto definitions at runtime.

## Quick Start
1. Install deps:
   ```bash
   cd sdk/node
   npm install
   ```
2. Run the sample worker:
   ```bash
   npm run build
   node dist/sample-worker.js
   ```

## Files
- `src/protos.ts` — loads CAP protos via protobufjs.
- `src/bus.ts` — NATS connector.
- `src/worker.ts` — worker skeleton.
- `src/client.ts` — submission helper.
- `src/sample-worker.ts` — minimal echo worker example.

## Notes
- Subjects: `sys.job.submit`, `job.<pool>`, `sys.job.result`, `sys.heartbeat`.
- Protocol version: `1`.
- Swap `bus.ts` for another transport if needed; keep message encoding via protobufjs or precompiled static modules (`pbjs/pbts`).
