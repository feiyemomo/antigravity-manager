import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  switchProfile,
  resolveProfile,
} from "../../src/core/profileSwitcher.js";
import {
  saveProfile,
  deleteProfile,
  getActiveProfileName,
  setActiveProfileName,
} from "../../src/core/profileManager.js";
import { credentialManager } from "../../src/core/credentialManager.js";
import { ProcessSafetyError, processInspector } from "../../src/core/processGuard.js";

describe("Profile Switcher & Atomicity", () => {
  const profileA = "test_switch_a_" + Date.now();
  const profileB = "test_switch_b_" + Date.now();

  beforeEach(() => {
    vi.restoreAllMocks();

    saveProfile(profileA, {
      installationId: "uuid-a-1111",
      tokenPayload: JSON.stringify({
        token: { access_token: "token-a", expiry: new Date(Date.now() + 100000).toISOString() },
        email: "alice@example.com",
      }),
      settingsPayload: JSON.stringify({ security: { auth: { selectedType: "oauth-personal" } } }),
      metadata: { name: profileA, email: "alice@example.com" },
    });

    saveProfile(profileB, {
      installationId: "uuid-b-2222",
      tokenPayload: JSON.stringify({
        token: { access_token: "token-b", expiry: new Date(Date.now() + 100000).toISOString() },
        email: "bob@example.com",
      }),
      settingsPayload: JSON.stringify({ security: { auth: { selectedType: "oauth-personal" } } }),
      metadata: { name: profileB, email: "bob@example.com" },
    });
  });

  afterEach(() => {
    deleteProfile(profileA);
    deleteProfile(profileB);
  });

  it("should resolve profile by name or email", () => {
    const resA = resolveProfile(profileA);
    expect(resA).not.toBeNull();
    expect(resA?.name).toBe(profileA);

    const resEmail = resolveProfile("bob@example.com");
    expect(resEmail).not.toBeNull();
    expect(resEmail?.name).toBe(profileB);
  });

  it("should refuse to switch if Antigravity is running (Process Safety)", async () => {
    vi.spyOn(processInspector, "listProcesses").mockReturnValue(
      `"Antigravity.exe","9999","Console","1","250,000 K"\n`
    );

    await expect(switchProfile(profileA)).rejects.toThrow(ProcessSafetyError);
  });

  it("should perform atomic switch when safe", async () => {
    vi.spyOn(credentialManager, "write").mockResolvedValue(true);
    vi.spyOn(credentialManager, "read").mockResolvedValue(JSON.stringify({ token: { access_token: "token-a" } }));

    const res = await switchProfile(profileA, { skipProcessCheck: true });
    expect(res.success).toBe(true);
    expect(res.profileName).toBe(profileA);
    expect(getActiveProfileName()).toBe(profileA);
  });

  it("should atomically roll back changes if credential manager write fails", async () => {
    setActiveProfileName(profileA);

    // Force credentialManager write to fail
    vi.spyOn(credentialManager, "write").mockResolvedValue(false);

    await expect(switchProfile(profileB, { skipProcessCheck: true })).rejects.toThrow(
      /ATOMIC SWITCH FAILED & ROLLED BACK/
    );

    // Active profile marker must be restored to original profileA
    expect(getActiveProfileName()).toBe(profileA);
  });
});
