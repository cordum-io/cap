# Troubleshooting Guide

Common issues when working with CAP and their solutions.

---

## 1. NATS Connection Refused

**Symptoms:**
```
connection refused: nats://127.0.0.1:4222
Error: Could not connect to server
```

**Cause:** NATS server is not running, or the URL/port is wrong.

**Solution:**

1. Start NATS:
   ```bash
   docker run -d --name nats -p 4222:4222 nats:latest
   ```

2. Verify it's running:
   ```bash
   docker ps | grep nats
   ```

3. If using a custom URL, set the environment variable:
   ```bash
   export NATS_URL=nats://your-host:4222
   ```

4. Check for auth mismatch — if your NATS server requires auth, pass credentials in the URL:
   ```
   nats://user:password@host:4222
   ```

---

## 2. Redis Connection Failed

**Symptoms:**
```
Error connecting to Redis: Connection refused
redis.exceptions.ConnectionError: Error 111 connecting to 127.0.0.1:6379
```

**Cause:** Redis is not running, or the URL format is wrong. Redis is only needed for the high-level runtime (pointer hydration), not the low-level SDK.

**Solution:**

1. Start Redis:
   ```bash
   docker run -d --name redis -p 6379:6379 redis:latest
   ```

2. Verify it's running:
   ```bash
   docker ps | grep redis
   redis-cli ping   # should return PONG
   ```

3. Set the correct URL format:
   ```bash
   export REDIS_URL=redis://127.0.0.1:6379/0
   ```
   Note: The URL must include the database number (`/0`).

4. If you only need the low-level SDK (worker/client without pointer hydration), Redis is not required.

---

## 3. Signature Verification Failed

**Symptoms:**
```
signature verification failed for sender <sender-id>
invalid signature: ECDSA verify failed
```

**Cause:** Key format mismatch, wrong key, or non-deterministic serialization.

**Solution:**

1. **Use P-256 (secp256r1) keys.** CAP requires ECDSA with the P-256 curve across all SDKs.

2. **Check key format per SDK:**

   - **Go:** Use `*ecdsa.PrivateKey` from `crypto/ecdsa` with `elliptic.P256()`:
     ```go
     priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
     ```

   - **Python:** Use `cryptography` with `ec.SECP256R1()`:
     ```python
     from cryptography.hazmat.primitives.asymmetric import ec
     priv = ec.generate_private_key(ec.SECP256R1())
     ```

   - **Node:** Use PEM-encoded PKCS#8 private key with `prime256v1`:
     ```bash
     node -e "const {generateKeyPairSync}=require('crypto');const {privateKey,publicKey}=generateKeyPairSync('ec',{namedCurve:'prime256v1',publicKeyEncoding:{type:'spki',format:'pem'},privateKeyEncoding:{type:'pkcs8',format:'pem'}});console.log(privateKey);console.log(publicKey);"
     ```

3. **Ensure the public key map matches sender IDs.** The `publicKeyMap`/`public_keys` must map sender IDs to the correct public keys. A mismatch causes verification failure.

4. **Cross-SDK verification** works because all SDKs use deterministic protobuf serialization (map entries sorted by key). If you're using a custom serializer, ensure it produces deterministic output.

---

## 4. Protobuf / Proto Errors

**Symptoms:**
```
proto: cannot parse invalid wire-format data
ModuleNotFoundError: No module named 'cap.pb.cordum.agent.v1'
Cannot find module '../cordum/agent/v1/job.proto'
```

**Cause:** Stale or missing generated protobuf stubs.

**Solution:**

1. **Regenerate stubs** from the repo root:
   ```bash
   ./tools/make_protos.sh
   ```

2. **For Python** stubs specifically:
   ```bash
   CAP_RUN_PY=1 ./tools/make_protos.sh
   ```
   Or manually:
   ```bash
   cd sdk/python
   python -m grpc_tools.protoc \
     -I../../proto \
     --python_out=./cap/pb \
     --grpc_python_out=./cap/pb \
     ../../proto/cordum/agent/v1/*.proto
   ```

3. **Check protoc version.** CAP proto files use proto3 syntax. Install `protoc` v3.19+ or use `grpc_tools.protoc` (Python) / `protobufjs` (Node).

4. **For Go**, stubs live at `cordum/agent/v1/` in the repo root. The import path is:
   ```go
   agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
   ```

5. **For Node**, protobufjs loads `.proto` files at runtime from `proto/`. If the proto files are missing, ensure you cloned the full repository.

---

## 5. Job Stuck in PENDING

**Symptoms:** Job is submitted but never picked up. No worker logs appear. The client gets no result.

**Cause:** No worker is subscribed to the job's topic/subject.

**Solution:**

1. **Check the topic matches.** The worker subscribes to a NATS subject (e.g., `job.echo`). The client's `JobRequest.topic` must match:
   ```
   Worker subject: "job.echo"
   JobRequest topic: "job.echo"   ✓
   JobRequest topic: "job.Echo"   ✗ (case-sensitive)
   ```

2. **Verify the worker is connected** using NATS monitoring:
   ```bash
   # List all subscriptions
   nats sub '>'
   ```

3. **Check the submit subject.** `client.Submit` publishes to `sys.job.submit` by default. If using the low-level worker, ensure it subscribes to the correct subject (the topic, not `sys.job.submit`).

4. **Check NATS queue groups.** Multiple workers on the same subject use competing consumers. If a worker is in a different queue group, it may not receive the message.

---

## 6. Worker Not Receiving Jobs

**Symptoms:** Worker is running but doesn't log any received jobs. Jobs appear to be submitted successfully.

**Cause:** Subject mismatch, wrong NATS connection, or queue group issue.

**Solution:**

1. **Verify the subject name** — must exactly match between client and worker:
   - **Go:** `Worker.Subject` field
   - **Python:** `subject` parameter in `run_worker()`
   - **Node:** `subject` in `startWorker()` config

2. **Check you're connected to the same NATS server.** If the worker connects to `nats://host-a:4222` and the client to `nats://host-b:4222`, messages won't route.

3. **Use NATS CLI to debug:**
   ```bash
   # Monitor all messages
   nats sub '>'

   # Check subscriptions
   nats server report connections
   ```

4. **Queue group issues:** CAP workers use `cap-workers` as the default queue group. If you have a custom queue group, ensure all workers for the same subject use the same group.

---

## 7. Import / Dependency Errors

**Symptoms:**
```
# Go
cannot find module providing package github.com/cordum-io/cap/v2/sdk/go/worker

# Python
ModuleNotFoundError: No module named 'cap'

# Node
Cannot find module 'cap-sdk-node'
```

**Cause:** Missing or incorrectly installed SDK package.

**Solution:**

**Go:**
```bash
# Make sure you use the /v2 module path
go get github.com/cordum-io/cap/v2@latest
go mod tidy
```
The Go module path is `github.com/cordum-io/cap/v2` — the `/v2` suffix is required.

**Python:**
```bash
pip install cap-sdk-python

# Or for development from source:
cd sdk/python
pip install -e .
```

**Node:**
```bash
npm install cap-sdk-node

# Or for development from source:
cd sdk/node
npm install
npm run build
```

---

## 8. Build Errors

**Symptoms:**
```
# Go
go: cannot find main module
go: no required module provides package ...

# Node
error TS2307: Cannot find module
tsc: error

# Python
google.protobuf.descriptor.FieldDescriptor has no attribute ...
```

**Cause:** Missing module initialization (Go), TypeScript config issues (Node), or stale proto stubs (Python).

**Solution:**

**Go — "cannot find main module":**
```bash
# Initialize a Go module in your project directory
go mod init my-project
go mod tidy
```

**Go — package not found after `go get`:**
```bash
# Ensure your go.mod requires the right version
go get github.com/cordum-io/cap/v2@latest
go mod tidy
```

**Node — TypeScript compilation errors:**
```bash
cd sdk/node
npm install
npm run build   # runs tsc
```
Check that `tsconfig.json` has `"moduleResolution": "node"` and targets ES2020+.

**Python — proto stubs out of date:**
```bash
# Regenerate from repo root
CAP_RUN_PY=1 ./tools/make_protos.sh

# Or manually
cd sdk/python
python -m grpc_tools.protoc \
  -I../../proto \
  --python_out=./cap/pb \
  --grpc_python_out=./cap/pb \
  ../../proto/cordum/agent/v1/*.proto
```

---

## 9. Heartbeat Not Sending

**Symptoms:** No heartbeat messages appear on `sys.heartbeat`. Monitoring shows the worker as offline.

**Cause:** Heartbeat interval not configured, or the worker crashed silently.

**Solution:**

1. **Check the worker is still running.** A crashed worker won't send heartbeats.

2. **For the low-level SDK**, heartbeats are manual. Use the heartbeat helpers:
   - **Go:** `capsdk.HeartbeatPayload()` or `capsdk.HeartbeatPayloadWithMemory()`
   - **Python:** Heartbeats are sent automatically by `run_worker()` if `heartbeat_interval` is set.
   - **Node:** Heartbeats are sent by `startWorker()` when configured.

3. **Monitor heartbeats:**
   ```bash
   nats sub sys.heartbeat
   ```

---

## 10. Envelope Decode Errors

**Symptoms:**
```
failed to unmarshal BusPacket
proto: cannot parse invalid wire-format data
malformed packet: missing required fields
```

**Cause:** The message on NATS is not a valid protobuf `BusPacket`, or the proto version is mismatched.

**Solution:**

1. **Don't publish raw JSON to NATS.** CAP uses binary protobuf encoding. All messages must be wrapped in a `BusPacket` envelope.

2. **Use the SDK's submit/publish functions** instead of raw NATS publish:
   - **Go:** `client.Submit()` or `worker.Worker.Start()`
   - **Python:** `client.submit_job()` or `worker.run_worker()`
   - **Node:** `submitJob()` or `startWorker()`

3. **Check proto version compatibility.** If you regenerated stubs with a different protoc version, ensure all SDKs use compatible stubs.

4. **Verify with conformance fixtures:**
   ```bash
   # Run conformance tests to verify proto encoding
   go test ./sdk/go/...
   cd sdk/python && python -m pytest tests/
   cd sdk/node && npm test
   ```
