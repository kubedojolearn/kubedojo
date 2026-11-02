// @vitest-environment node
import { describe, expect, it } from "vitest";
import { asgardeoSubject, provisionProfile, testUserId } from "@/lib/auth/identity";

describe("asgardeoSubject", () => {
  it("accepts Asgardeo user IDs, normalized to lowercase", () => {
    expect(asgardeoSubject("5D1B7C1E-8F43-4C2A-9A51-3E0F1D2C4B6A")).toBe(
      "asgardeo:5d1b7c1e-8f43-4c2a-9a51-3e0f1d2c4b6a",
    );
  });

  it.each([["octo@example.com"], ["octocat"], ["583231"], [""], [undefined], [42]])("rejects %s", (sub) => {
    expect(asgardeoSubject(sub)).toBeNull();
  });
});

describe("provisionProfile", () => {
  it("uses a handle-shaped username, the name and the picture", () => {
    expect(
      provisionProfile({
        preferred_username: "octocat",
        name: "The Octocat",
        picture: "https://avatars.githubusercontent.com/u/583231",
      }),
    ).toEqual({ login: "octocat", name: "The Octocat", avatar_url: "https://avatars.githubusercontent.com/u/583231" });
  });

  it("never passes an email address on, as login or name", () => {
    const p = provisionProfile({
      preferred_username: "ada@example.com",
      username: "ada@example.com",
      name: "ada@example.com",
    });
    expect(JSON.stringify(p)).not.toContain("@");
    expect(p).toEqual({ login: "learner", name: "learner", avatar_url: "" });
  });

  it("derives a handle from the name when the username isn't usable", () => {
    expect(
      provisionProfile({ preferred_username: "ada@example.com", given_name: "Zoë", family_name: "O'Brien" }),
    ).toEqual({ login: "zoe-o-brien", name: "Zoë O'Brien", avatar_url: "" });
  });

  it("turns an unusual username into a handle rather than dropping it", () => {
    expect(provisionProfile({ preferred_username: "ada.lovelace" }).login).toBe("ada-lovelace");
  });

  it("keeps handles to 39 characters", () => {
    expect(provisionProfile({ name: "a".repeat(30) + " " + "b".repeat(30) }).login).toHaveLength(39);
  });
});

describe("testUserId", () => {
  it("maps the same test ID to the same UUID-shaped ID", async () => {
    const a = await testUserId("1001");
    expect(a).toBe(await testUserId("1001"));
    expect(a).not.toBe(await testUserId("1002"));
    expect(asgardeoSubject(a)).toBe(`asgardeo:${a}`);
  });
});
