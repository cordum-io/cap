"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
const client_1 = require("../src/client");
const worker_1 = require("../src/worker");
const protos_1 = require("../src/protos");
const crypto = __importStar(require("crypto"));
// Mock NatsConnection
class MockNatsConnection {
    constructor() {
        this.closed = Promise.resolve();
        this.info = { client_id: 123 };
        this.isClosed = false;
        this.isDraining = false;
        this.isReconnecting = false;
        this.isConnecting = false;
        this.isUpdating = false;
        this.options = {};
        this.status = Promise.resolve();
        this._publishedMessages = [];
        this._subscriptions = new Map();
        // Implement other NatsConnection methods as needed for your tests
    }
    async publish(subject, data) {
        this._publishedMessages.push({ subject, data });
    }
    subscribe(subject, opts) {
        const callback = (msg) => {
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
            drain: async () => { },
            unsubscribe: () => { },
            setMaxPending: () => { },
            sid: 1, // Dummy value
        };
    }
    async flush() { }
    async request(subject, data, opts) {
        return {};
    }
    async close() { }
    async drain() { }
}
describe("CAP SDK E2E Test", () => {
    let clientPrivateKey;
    let clientPublicKey;
    let workerPrivateKey;
    let workerPublicKey;
    beforeAll(() => {
        // Generate client keys
        const clientKey = crypto.generateKeyPairSync("ec", {
            namedCurve: "secp256k1",
            publicKeyEncoding: { type: "spki", format: "pem" },
            privateKeyEncoding: { type: "pkcs8", format: "pem" },
        });
        clientPrivateKey = clientKey.privateKey;
        clientPublicKey = clientKey.publicKey;
        // Generate worker keys
        const workerKey = crypto.generateKeyPairSync("ec", {
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
        await (0, worker_1.startWorker)({
            nc: mockNats,
            subject: testSubject,
            handler: async (jobRequest) => {
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
        await (0, client_1.submitJob)(mockNats, jobRequest, "trace-abc", "test-client", clientPrivateKey);
        // Expect client to publish a message
        expect(mockNats._publishedMessages.length).toBe(1);
        const clientPublishedMsg = mockNats._publishedMessages[0];
        expect(clientPublishedMsg.subject).toBe("sys.job.submit");
        // Simulate NATS delivering the message to the worker
        const workerCallback = mockNats._subscriptions.get(testSubject);
        expect(workerCallback).toBeDefined();
        if (workerCallback) {
            await workerCallback(clientPublishedMsg);
        }
        else {
            fail("Worker did not subscribe correctly");
        }
        // Expect worker to publish a result message
        expect(mockNats._publishedMessages.length).toBe(2);
        const workerPublishedMsg = mockNats._publishedMessages[1];
        expect(workerPublishedMsg.subject).toBe("sys.job.result");
        // Verify worker's published message signature
        const root = await (0, protos_1.loadRoot)();
        const BusPacket = root.lookupType("coretex.agent.v1.BusPacket");
        const resultPacket = BusPacket.decode(workerPublishedMsg.data);
        expect(resultPacket.signature).toBeDefined();
        const receivedSignature = resultPacket.signature;
        resultPacket.signature = Buffer.from([]); // Clear signature for verification
        const unsignedData = BusPacket.encode(resultPacket).finish();
        const verify = crypto.createVerify("sha256");
        verify.update(unsignedData);
        expect(verify.verify(workerPublicKey, receivedSignature)).toBe(true);
        expect(resultPacket.job_result.job_id).toBe("job-123");
        expect(resultPacket.job_result.status).toBe("JOB_STATUS_SUCCEEDED");
    });
});
