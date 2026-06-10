// @ts-nocheck — semgrep test-no-skipped-tests fixture

// ─── UNSAFE ───────────────────────────────────────────────────────────────────

describe.skip("slow suite", () => {
  it("works", () => { expect(1).toBe(1); });
});

describe("feature", () => {
  it.skip("flaky test", () => {
    expect(Math.random()).toBeGreaterThan(0);
  });

  test.skip("unstable integration", async () => {
    await fetch("https://example.com/api");
  });
});

describe("when feature flag is off", () => {
  it("does nothing", () => {});
});

// ─── SAFE (no .skip()) ────────────────────────────────────────────────────────

describe("payment suite", () => {
  it("charges card", () => { expect(true).toBe(true); });
  test("refunds", () => { expect(true).toBe(true); });
});
