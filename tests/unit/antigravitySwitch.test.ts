import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { tokenStore } from "../../src/server/services/tokenStore.js";
import { ANTIGRAVITY_TOKEN_FILE } from "../../src/shared/constants.js";
import { existsSync, readFileSync, unlinkSync, writeFileSync } from "node:fs";

describe("Antigravity Account Switch & Rotate", () => {
  it("should have switchAntigravityAccount and rotateAntigravityAccount methods", () => {
    expect(typeof tokenStore.switchAntigravityAccount).toBe("function");
    expect(typeof tokenStore.rotateAntigravityAccount).toBe("function");
  });

  it("should detect current account from store", async () => {
    await tokenStore.load();
    const currentId = tokenStore.getCurrentAccountId();
    // Either undefined or string
    expect(currentId === undefined || typeof currentId === "string").toBe(true);
  });
});
