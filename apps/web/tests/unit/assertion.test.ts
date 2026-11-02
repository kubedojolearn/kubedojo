// @vitest-environment node
import { generateKeyPairSync } from "node:crypto";
import { decodeProtectedHeader, importSPKI, jwtVerify } from "jose";
import { beforeAll, describe, expect, it } from "vitest";
import { AUDIENCE_API, AUDIENCE_PROVISION, mintAssertion } from "@/lib/auth/assertion";

const USER = "asgardeo:5d1b7c1e-8f43-4c2a-9a51-3e0f1d2c4b6a";

let publicPem: string;

beforeAll(() => {
  const { privateKey, publicKey } = generateKeyPairSync("ed25519");
  process.env.BFF_SIGNING_PRIVATE_KEY = privateKey.export({ format: "der", type: "pkcs8" }).toString("base64");
  process.env.BFF_SIGNING_KID = "unit-1";
  publicPem = publicKey.export({ format: "pem", type: "spki" }).toString();
});

describe("mintAssertion", () => {
  it("produces an EdDSA JWT matching the Go API contract", async () => {
    const token = await mintAssertion(USER, AUDIENCE_API);
    expect(decodeProtectedHeader(token)).toEqual({ alg: "EdDSA", kid: "unit-1", typ: "JWT" });
    const { payload } = await jwtVerify(token, await importSPKI(publicPem, "EdDSA"), {
      issuer: "kubedojo-web",
      audience: "kubedojo-api",
    });
    expect(payload.sub).toBe(USER);
    expect(payload.exp! - payload.iat!).toBe(60);
    expect(payload.jti).toMatch(/^[0-9a-f-]{36}$/);
    expect(payload).not.toHaveProperty("login");
  });

  it("puts profile claims only on provisioning assertions", async () => {
    const token = await mintAssertion(USER, AUDIENCE_PROVISION, { login: "octo", name: "Octo", avatar_url: "" });
    const { payload } = await jwtVerify(token, await importSPKI(publicPem, "EdDSA"), { audience: AUDIENCE_PROVISION });
    expect(payload.login).toBe("octo");
    await expect(mintAssertion(USER, AUDIENCE_PROVISION)).rejects.toThrow();
    await expect(mintAssertion(USER, AUDIENCE_API, { login: "x", name: "x", avatar_url: "" })).rejects.toThrow();
  });

  it("refuses subjects that are not immutable Asgardeo user IDs", async () => {
    for (const bad of [
      "asgardeo:octocat",
      "asgardeo:octo@example.com",
      "asgardeo:5D1B7C1E-8F43-4C2A-9A51-3E0F1D2C4B6A",
      "5d1b7c1e-8f43-4c2a-9a51-3e0f1d2c4b6a",
      "asgardeo:",
      "github:583231",
    ]) {
      await expect(mintAssertion(bad, AUDIENCE_API)).rejects.toThrow();
    }
  });

  it("issues a unique jti per assertion", async () => {
    const a = await mintAssertion(USER, AUDIENCE_API);
    const b = await mintAssertion(USER, AUDIENCE_API);
    expect(a).not.toBe(b);
  });
});
