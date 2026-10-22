import { getBuildInfo } from "@/lib/build-info";

describe("getBuildInfo", () => {
  const saved = { ...process.env };
  afterEach(() => {
    process.env = { ...saved };
  });

  it("falls back to dev values", () => {
    delete process.env.KUBEDOJO_VERSION;
    delete process.env.KUBEDOJO_COMMIT;
    expect(getBuildInfo()).toEqual({ version: "dev", commit: "unknown" });
  });

  it("reports injected build metadata and nothing else", () => {
    process.env.KUBEDOJO_VERSION = "1.2.3";
    process.env.KUBEDOJO_COMMIT = "abc1234";
    process.env.AUTH_SECRET = "must-not-leak";
    expect(getBuildInfo()).toEqual({ version: "1.2.3", commit: "abc1234" });
  });
});
