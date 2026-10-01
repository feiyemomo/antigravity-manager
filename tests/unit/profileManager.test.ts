import { describe, it, expect, beforeEach, afterEach } from "vitest";
import {
  initProfilesDirectory,
  saveProfile,
  getProfile,
  listProfiles,
  deleteProfile,
  setActiveProfileName,
  getActiveProfileName,
  extractEmailFromTokenPayload,
  extractExpiryFromTokenPayload,
} from "../../src/core/profileManager.js";
import { PROFILES_DIR } from "../../src/core/constants.js";
import { existsSync, rmSync } from "node:fs";
import { join } from "node:path";

describe("Profile Manager", () => {
  const testProfileName = "test_profile_unit_" + Date.now();

  beforeEach(() => {
    initProfilesDirectory();
  });

  afterEach(() => {
    deleteProfile(testProfileName);
  });

  it("should extract email and expiry from token payload", () => {
    const payload = {
      token: {
        access_token: "ya29.test",
        expiry: "2026-09-30T12:00:00.000Z",
      },
      email: "test@example.com",
    };

    expect(extractEmailFromTokenPayload(payload)).toBe("test@example.com");
    expect(extractExpiryFromTokenPayload(payload)).toBe(Date.parse("2026-09-30T12:00:00.000Z"));
  });

  it("should save and retrieve an isolated profile with installation_id, token, and settings", () => {
    const tokenObj = {
      token: {
        access_token: "ya29.mock",
        token_type: "Bearer",
        refresh_token: "1//mock",
        expiry: new Date(Date.now() + 3600000).toISOString(),
      },
      email: "mock@example.com",
    };

    const saved = saveProfile(testProfileName, {
      installationId: "test-install-uuid-1234",
      tokenPayload: JSON.stringify(tokenObj, null, 2),
      settingsPayload: JSON.stringify({ security: { auth: { selectedType: "oauth-personal" } } }),
      metadata: {
        email: "mock@example.com",
        tier: "FREE",
      },
    });

    expect(saved.name).toBe(testProfileName);
    expect(saved.email).toBe("mock@example.com");
    expect(saved.hasInstallationId).toBe(true);
    expect(saved.hasToken).toBe(true);
    expect(saved.hasSettings).toBe(true);

    // Verify files on disk
    const profileDir = join(PROFILES_DIR, testProfileName);
    expect(existsSync(join(profileDir, "installation_id"))).toBe(true);
    expect(existsSync(join(profileDir, "antigravity-oauth-token"))).toBe(true);
    expect(existsSync(join(profileDir, "settings.json"))).toBe(true);

    // Retrieve via getProfile
    const retrieved = getProfile(testProfileName);
    expect(retrieved).not.toBeNull();
    expect(retrieved?.installationId).toBe("test-install-uuid-1234");
    expect(JSON.parse(retrieved!.tokenPayload).email).toBe("mock@example.com");
  });

  it("should set and get active profile marker", () => {
    setActiveProfileName(testProfileName);
    expect(getActiveProfileName()).toBe(testProfileName);
  });

  it("should list profiles and flag the active one", () => {
    saveProfile(testProfileName, {
      installationId: "uuid-1",
      tokenPayload: JSON.stringify({ token: { access_token: "token1" }, email: "mock@test.com" }),
      settingsPayload: "{}",
    });

    setActiveProfileName(testProfileName);
    const profiles = listProfiles();
    const found = profiles.find((p) => p.name === testProfileName);

    expect(found).toBeDefined();
    expect(found?.isActive).toBe(true);
  });

  it("should delete profile cleanly", () => {
    saveProfile(testProfileName, {
      installationId: "uuid-to-del",
      tokenPayload: "{}",
      settingsPayload: "{}",
    });

    const deleted = deleteProfile(testProfileName);
    expect(deleted).toBe(true);

    const profileDir = join(PROFILES_DIR, testProfileName);
    expect(existsSync(profileDir)).toBe(false);
  });
});
