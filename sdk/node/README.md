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
- Signing: `submitJob` and `startWorker` sign envelopes when given a PEM private key; set `publicKeyMap` to verify incoming packets. Generate a P-256 keypair with:
  ```bash
  node -e "const {generateKeyPairSync}=require('crypto');const {privateKey,publicKey}=generateKeyPairSync('ec',{namedCurve:'prime256v1',publicKeyEncoding:{type:'spki',format:'pem'},privateKeyEncoding:{type:'pkcs8',format:'pem'}});console.log(privateKey);console.log(publicKey);"
  ```
