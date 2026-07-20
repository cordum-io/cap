import { expect } from "chai";
import fs from "fs";
import path from "path";
import {
  extractProductionSignature,
  productionPreimageDigest,
  verifyProductionSignature,
} from "../src/production-signing";

describe("CAP-PRODUCTION raw signing vectors", () => {
  const fixture = JSON.parse(
    fs.readFileSync(path.resolve(__dirname, "../../../../test/fixtures/production-signing-v1.json"), "utf8"),
  ) as Record<string, string>;

  it("verifies the Go producer without protobuf reserialization", () => {
    const raw = Buffer.from(fixture.raw_base64, "base64");
    const extracted = extractProductionSignature(raw);

    expect(Buffer.from(extracted.unsigned).toString("base64")).to.equal(fixture.unsigned_base64);
    expect(Buffer.from(productionPreimageDigest(extracted.unsigned)).toString("hex")).to.equal(
      fixture.preimage_digest_hex,
    );
    expect(Buffer.from(verifyProductionSignature(raw, fixture.public_key_pem)).toString("base64")).to.equal(
      fixture.unsigned_base64,
    );
  });

  it("rejects a duplicate signature field", () => {
    const raw = Buffer.from(fixture.raw_base64, "base64");
    const duplicate = Buffer.concat([raw, Buffer.from([0x72, 0x01, 0x00])]);
    expect(() => extractProductionSignature(duplicate)).to.throw("duplicate");
  });
});
