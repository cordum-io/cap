import { NatsConnection, Msg, Subscription } from "nats";
import { submitJob } from "../src/client";
import { startWorker } from "../src/worker";
import { loadRoot } from "../src/protos";
import * as crypto from "crypto";
import Long from "long";
import { expect } from "chai"; // Import expect from chai

// Mock NatsConnection
class MockNatsConnection {
  _publishedMessages: Msg[] = [];
  _subscriptions: Map<string, (msg: Msg) => void> = new Map();

  async publish(subject: string, data: Uint8Array): Promise<void> {
    this._publishedMessages.push({ subject, data } as Msg);
  }

  subscribe(subject: string, opts?: any): Subscription {
    const callback = (msg: Msg) => {
      // Simulate NATS message delivery
    };
    this._subscriptions.set(subject, callback);
    return {
      getSubject: () => subject,
      getReceived: () => 1, // Dummy value
      getProcessed: () => 1, // Dummy value
      getPending: () => 0, // Dummy value
      isDraining: () => false,
      isClosed: () => false,
      drain: async () => {},
      unsubscribe: () => {},
    };
  }
}

describe("CAP SDK E2E Test", () => {
  let clientPrivateKey: string;
  let clientPublicKey: string;
  let workerPrivateKey: string;
  let workerPublicKey: string;

  before(() => {
    // Generate client keys
    const clientKey = crypto.generateKeyPairSync("ec", { // Explicitly set type to "ec"
      namedCurve: "secp256k1",
      publicKeyEncoding: { type: "spki", format: "pem" },
      privateKeyEncoding: { type: "pkcs8", format: "pem" },
    });
    clientPrivateKey = clientKey.privateKey;
    clientPublicKey = clientKey.publicKey;

    // Generate worker keys
    const workerKey = crypto.generateKeyPairSync("ec", { // Explicitly set type to "ec"
      namedCurve: "secp256k1",
      publicKeyEncoding: { type: "spki", format: "pem" },
      privateKeyEncoding: { type: "pkcs8", format: "pem" },
    });
    workerPrivateKey = workerKey.privateKey;
    workerPublicKey = workerKey.publicKey;
  });

  it("should successfully submit and process a signed job", async () => {
    const mockNats = new MockNatsConnection();
    const testSubject = "test.worker.queue";

    // Start the worker
    await startWorker({
      nc: mockNats as any, // Cast to any to bypass type checking for the mock
      subject: testSubject,
      handler: async (jobRequest: any) => {
        return {
          job_id: jobRequest.job_id,
          status: "JOB_STATUS_SUCCEEDED",
          result_ptr: "s3://test-bucket/result",
        };
      },
      publicKeyMap: { "test-client": clientPublicKey },
      privateKey: workerPrivateKey,
      senderId: "test-worker",
    });

    // Simulate client submitting a job
    const jobRequest = {
      job_id: "job-123",
      topic: testSubject,
      context_ptr: "s3://test-bucket/context",
    };

    await submitJob(mockNats as any, jobRequest, "trace-abc", "test-client", clientPrivateKey);

    // Expect client to publish a message
    expect(mockNats._publishedMessages).to.have.lengthOf(1);
    const clientPublishedMsg = mockNats._publishedMessages[0];
    expect(clientPublishedMsg.subject).to.equal("sys.job.submit");

    // Simulate NATS delivering the message to the worker
    const workerCallback = mockNats._subscriptions.get(testSubject);
    expect(workerCallback).to.exist;
    if (workerCallback) {
      await workerCallback(clientPublishedMsg);
    } else {
        throw new Error("Worker did not subscribe correctly");
    }

    // Expect worker to publish a result message
    expect(mockNats._publishedMessages).to.have.lengthOf(2);
    const workerPublishedMsg = mockNats._publishedMessages[1];
    expect(workerPublishedMsg.subject).to.equal("sys.job.result");

    // Verify worker's published message signature
    const root = await loadRoot();
    const BusPacket = root.lookupType("cortex.agent.v1.BusPacket");

    const resultPacket = BusPacket.decode(workerPublishedMsg.data) as any;
    expect(resultPacket.signature).to.exist;

    const receivedSignature = resultPacket.signature;
    resultPacket.signature = Buffer.from([]); // Clear signature for verification

    const unsignedData = BusPacket.encode(resultPacket).finish();
    const verify = crypto.createVerify("sha256");
    verify.update(unsignedData);
    expect(verify.verify(workerPublicKey, receivedSignature)).to.be.true;

    expect(resultPacket.job_result.job_id).to.equal("job-123");
    expect(resultPacket.job_result.status).to.equal("JOB_STATUS_SUCCEEDED");
  });
});
