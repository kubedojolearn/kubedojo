// @vitest-environment node
import { generateKeyPairSync } from "node:crypto";
import { decodeProtectedHeader, importSPKI, jwtVerify } from "jose";
import { beforeAll, describe, expect, it } from "vitest";
import { AUDIENCE_API, AUDIENCE_PROVISION, mintAssertion } from "@/lib/auth/assertion";

let publicPem: string;

beforeAll(() => {
  const { privateKey, publicKey } = generateKeyPairSync("ed25519");
  process.env.BFF_SIGNING_PRIVATE_KEY = privateKey.export({ format: "der", type: "pkcs8" }).toString("base64");
  process.env.BFF_SIGNING_KID = "unit-1";
  publicPem = publicKey.export({ format: "pem", type: "spki" }).toString();
});

describe("mintAssertion", () => {
  it("produces an EdDSA JWT matching the Go API contract", async () => {
    const token = await mintAssertion("github:583231", AUDIENCE_API);
    expect(decodeProtectedHeader(token)).toEqual({ alg: "EdDSA", kid: "unit-1", typ: "JWT" });
    const { payload } = await jwtVerify(token, await importSPKI(publicPem, "EdDSA"), {
      issuer: "kubedojo-web",
      audience: "kubedojo-api",
    });
    expect(payload.sub).toBe("github:583231");
    expect(payload.exp! - payload.iat!).toBe(60);
    expect(payload.jti).toMatch(/^[0-9a-f-]{36}$/);
    expect(payload).not.toHaveProperty("login");
  });

  it("puts profile claims only on provisioning assertions", async () => {
    const token = await mintAssertion("github:1", AUDIENCE_PROVISION, { login: "octo", name: "Octo", avatar_url: "" });
    const { payload } = await jwtVerify(token, await importSPKI(publicPem, "EdDSA"), { audience: AUDIENCE_PROVISION });
    expect(payload.login).toBe("octo");
    await expect(mintAssertion("github:1", AUDIENCE_PROVISION)).rejects.toThrow();
    await expect(mintAssertion("github:1", AUDIENCE_API, { login: "x", name: "x", avatar_url: "" })).rejects.toThrow();
  });

  it("refuses subjects that are not immutable numeric GitHub IDs", async () => {
    for (const bad of ["github:octocat", "octocat", "github:0123", "github:", "gitlab:1"]) {
      await expect(mintAssertion(bad, AUDIENCE_API)).rejects.toThrow();
    }
  });

  it("issues a unique jti per assertion", async () => {
    const a = await mintAssertion("github:1", AUDIENCE_API);
    const b = await mintAssertion("github:1", AUDIENCE_API);
    expect(a).not.toBe(b);
  });
});
