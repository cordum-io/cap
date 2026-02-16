import assert from "node:assert";
import {
  CAPError,
  MalformedPacketError,
  SignatureInvalidError,
  JobTimeoutError,
  InvalidInputError,
  SafetyDeniedError,
} from "../src/errors";

describe("CAP typed errors", () => {
  it("MalformedPacketError is instanceof CAPError and Error", () => {
    const err = new MalformedPacketError("bad wire bytes");
    assert.ok(err instanceof CAPError);
    assert.ok(err instanceof Error);
    assert.strictEqual(err.code, "ERROR_CODE_PROTOCOL_MALFORMED_PACKET");
    assert.strictEqual(err.numericCode, 101);
    assert.strictEqual(err.message, "bad wire bytes");
    assert.strictEqual(err.name, "MalformedPacketError");
  });

  it("subclasses have correct codes", () => {
    const cases: Array<[CAPError, string, number]> = [
      [new SignatureInvalidError("bad"), "ERROR_CODE_PROTOCOL_SIGNATURE_INVALID", 103],
      [new JobTimeoutError("slow"), "ERROR_CODE_JOB_TIMEOUT", 200],
      [new InvalidInputError("missing"), "ERROR_CODE_JOB_INVALID_INPUT", 203],
      [new SafetyDeniedError("blocked"), "ERROR_CODE_SAFETY_DENIED", 300],
    ];

    for (const [err, code, numericCode] of cases) {
      assert.strictEqual(err.code, code);
      assert.strictEqual(err.numericCode, numericCode);
      assert.ok(err instanceof CAPError);
    }
  });

  it("can be caught with try/catch by type", () => {
    try {
      throw new MalformedPacketError("test");
    } catch (err) {
      assert.ok(err instanceof MalformedPacketError);
      assert.ok(err instanceof CAPError);
      assert.ok(err instanceof Error);
    }
  });
});
