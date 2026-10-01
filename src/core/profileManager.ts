import {
  existsSync,
  mkdirSync,
  readdirSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { join } from "node:path";
import { randomUUID } from "node:crypto";
import {
  ACTIVE_PROFILE_FILE,
  AGY_AUTH_DIR,
  PROFILES_DIR,
  PRIMARY_INSTALLATION_ID_FILE,
  PRIMARY_TOKEN_FILE,
  PRIMARY_SETTINGS_FILE,
  SECONDARY_TOKEN_FILE,
} from "./constants.js";
import { credentialManager } from "./credentialManager.js";
import type { IsolatedFilesPayload, ProfileInfo, ProfileMetadata } from "./types.js";

/**
 * Initializes the profiles directory structure
 */
export function initProfilesDirectory(): void {
  if (!existsSync(AGY_AUTH_DIR)) {
    mkdirSync(AGY_AUTH_DIR, { recursive: true });
  }
  if (!existsSync(PROFILES_DIR)) {
    mkdirSync(PROFILES_DIR, { recursive: true });
  }
}

/**
 * Gets the name of the currently active profile
 */
export function getActiveProfileName(): string | null {
  try {
    if (existsSync(ACTIVE_PROFILE_FILE)) {
      const name = readFileSync(ACTIVE_PROFILE_FILE, "utf-8").trim();
      return name || null;
    }
  } catch {
    // ignore
  }
  return null;
}

/**
 * Sets the active profile name in ~/.agy_auth/active_profile
 */
export function setActiveProfileName(name: string): void {
  initProfilesDirectory();
  writeFileSync(ACTIVE_PROFILE_FILE, name.trim(), "utf-8");
}

/**
 * Gets the profile directory path for a given profile name
 */
export function getProfileDir(name: string): string {
  return join(PROFILES_DIR, name);
}

/**
 * Extracts email from token payload or ID token JWT
 */
export function extractEmailFromTokenPayload(payload: any): string | undefined {
  if (!payload) return undefined;

  // 1. Direct email field
  if (typeof payload.email === "string" && payload.email) {
    return payload.email;
  }

  // 2. In userinfo / user
  if (payload.user && typeof payload.user.email === "string") {
    return payload.user.email;
  }

  // 3. In id_token JWT
  const idToken = payload.id_token || payload.idToken || (payload.token && payload.token.id_token);
  if (typeof idToken === "string" && idToken.includes(".")) {
    try {
      const parts = idToken.split(".");
      if (parts.length >= 2) {
        const decoded = Buffer.from(parts[1], "base64url").toString("utf-8");
        const jwtPayload = JSON.parse(decoded);
        if (jwtPayload.email) return jwtPayload.email;
      }
    } catch {
      // ignore
    }
  }

  return undefined;
}

/**
 * Parses expiry timestamp from token payload
 */
export function extractExpiryFromTokenPayload(payload: any): number | undefined {
  if (!payload) return undefined;

  // token.expiry (ISO string or unix ms)
  const expiry = payload.token?.expiry || payload.expiry || payload.expiresAt;
  if (typeof expiry === "number") {
    return expiry;
  }
  if (typeof expiry === "string") {
    const parsed = Date.parse(expiry);
    if (!isNaN(parsed)) return parsed;
  }

  return undefined;
}

/**
 * Lists all registered profiles and their health
 */
export function listProfiles(): ProfileInfo[] {
  initProfilesDirectory();

  if (!existsSync(PROFILES_DIR)) {
    return [];
  }

  const activeName = getActiveProfileName();
  const entries = readdirSync(PROFILES_DIR, { withFileTypes: true });
  const profiles: ProfileInfo[] = [];

  for (const entry of entries) {
    if (!entry.isDirectory()) continue;
    const name = entry.name;
    const profileDir = join(PROFILES_DIR, name);

    const installFile = join(profileDir, "installation_id");
    const tokenFile = join(profileDir, "antigravity-oauth-token");
    const jetskiFile = join(profileDir, "jetski-standalone-oauth-token");
    const settingsFile = join(profileDir, "settings.json");
    const metaFile = join(profileDir, "profile.json");

    const hasInstallationId = existsSync(installFile);
    const hasToken = existsSync(tokenFile) || existsSync(jetskiFile);
    const hasSettings = existsSync(settingsFile);

    let metadata: Partial<ProfileMetadata> = {};
    if (existsSync(metaFile)) {
      try {
        metadata = JSON.parse(readFileSync(metaFile, "utf-8"));
      } catch {
        // ignore
      }
    }

    let tokenExpiresAt: number | undefined;
    let email = metadata.email;

    if (hasToken) {
      try {
        const targetTokenFile = existsSync(tokenFile) ? tokenFile : jetskiFile;
        const parsedToken = JSON.parse(readFileSync(targetTokenFile, "utf-8"));
        if (!email) {
          email = extractEmailFromTokenPayload(parsedToken);
        }
        tokenExpiresAt = extractExpiryFromTokenPayload(parsedToken);
      } catch {
        // ignore
      }
    }

    const isTokenExpired = tokenExpiresAt !== undefined ? tokenExpiresAt < Date.now() : undefined;

    profiles.push({
      name,
      email,
      accountId: metadata.accountId,
      createdAt: metadata.createdAt || new Date().toISOString(),
      updatedAt: metadata.updatedAt || new Date().toISOString(),
      tier: metadata.tier,
      lastUsedAt: metadata.lastUsedAt,
      isActive: name === activeName,
      hasInstallationId,
      hasToken,
      hasSettings,
      tokenExpiresAt,
      isTokenExpired,
    });
  }

  return profiles;
}

/**
 * Reads isolated files for a specific profile
 */
export function getProfile(name: string): IsolatedFilesPayload | null {
  const profileDir = getProfileDir(name);
  if (!existsSync(profileDir)) {
    return null;
  }

  const installFile = join(profileDir, "installation_id");
  const tokenFile = join(profileDir, "antigravity-oauth-token");
  const jetskiFile = join(profileDir, "jetski-standalone-oauth-token");
  const settingsFile = join(profileDir, "settings.json");
  const metaFile = join(profileDir, "profile.json");

  if (!existsSync(installFile)) return null;

  const actualTokenFile = existsSync(tokenFile) ? tokenFile : existsSync(jetskiFile) ? jetskiFile : null;
  if (!actualTokenFile) return null;

  try {
    const installationId = readFileSync(installFile, "utf-8").trim();
    const tokenPayload = readFileSync(actualTokenFile, "utf-8");
    const settingsPayload = existsSync(settingsFile)
      ? readFileSync(settingsFile, "utf-8")
      : JSON.stringify({ mcpServers: {}, security: { auth: { selectedType: "oauth-personal" } } }, null, 2);

    let metadata: Partial<ProfileMetadata> = {};
    if (existsSync(metaFile)) {
      try {
        metadata = JSON.parse(readFileSync(metaFile, "utf-8"));
      } catch {
        // ignore
      }
    }

    return {
      installationId,
      tokenPayload,
      settingsPayload,
      metadata,
    };
  } catch {
    return null;
  }
}

/**
 * Saves or updates a profile in ~/.agy_auth/profiles/<name>/
 */
export function saveProfile(
  name: string,
  payload: IsolatedFilesPayload
): ProfileInfo {
  initProfilesDirectory();
  const profileDir = getProfileDir(name);
  if (!existsSync(profileDir)) {
    mkdirSync(profileDir, { recursive: true });
  }

  const installFile = join(profileDir, "installation_id");
  const tokenFile = join(profileDir, "antigravity-oauth-token");
  const jetskiFile = join(profileDir, "jetski-standalone-oauth-token");
  const settingsFile = join(profileDir, "settings.json");
  const metaFile = join(profileDir, "profile.json");

  writeFileSync(installFile, payload.installationId.trim(), "utf-8");
  writeFileSync(tokenFile, payload.tokenPayload.trim(), "utf-8");
  writeFileSync(jetskiFile, payload.tokenPayload.trim(), "utf-8");
  writeFileSync(settingsFile, payload.settingsPayload.trim(), "utf-8");

  let parsedToken: any = null;
  try {
    parsedToken = JSON.parse(payload.tokenPayload);
  } catch {
    // ignore
  }

  const email = payload.metadata?.email || extractEmailFromTokenPayload(parsedToken);
  const tokenExpiresAt = extractExpiryFromTokenPayload(parsedToken);

  const metadata: ProfileMetadata = {
    name,
    email,
    accountId: payload.metadata?.accountId,
    createdAt: payload.metadata?.createdAt || new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    tier: payload.metadata?.tier || "FREE",
    lastUsedAt: payload.metadata?.lastUsedAt || Date.now(),
  };

  writeFileSync(metaFile, JSON.stringify(metadata, null, 2), "utf-8");

  return {
    ...metadata,
    isActive: name === getActiveProfileName(),
    hasInstallationId: true,
    hasToken: true,
    hasSettings: true,
    tokenExpiresAt,
    isTokenExpired: tokenExpiresAt !== undefined ? tokenExpiresAt < Date.now() : undefined,
  };
}

/**
 * Captures the current active Antigravity environment files & credentials into a new profile
 */
export async function captureCurrentAsProfile(name: string): Promise<ProfileInfo> {
  // 1. Installation ID
  let installationId = "";
  if (existsSync(PRIMARY_INSTALLATION_ID_FILE)) {
    installationId = readFileSync(PRIMARY_INSTALLATION_ID_FILE, "utf-8").trim();
  }
  if (!installationId) {
    installationId = randomUUID();
  }

  // 2. Token payload: try Credential Manager first, then local files
  let tokenPayload = "";
  const credSecret = await credentialManager.read();
  if (credSecret) {
    tokenPayload = credSecret;
  } else if (existsSync(PRIMARY_TOKEN_FILE)) {
    tokenPayload = readFileSync(PRIMARY_TOKEN_FILE, "utf-8");
  } else if (existsSync(SECONDARY_TOKEN_FILE)) {
    tokenPayload = readFileSync(SECONDARY_TOKEN_FILE, "utf-8");
  }

  if (!tokenPayload) {
    throw new Error(
      "No active Antigravity token found in Credential Manager or ~/.gemini/jetski-standalone-oauth-token."
    );
  }

  // 3. Settings
  let settingsPayload = "";
  if (existsSync(PRIMARY_SETTINGS_FILE)) {
    settingsPayload = readFileSync(PRIMARY_SETTINGS_FILE, "utf-8");
  } else {
    settingsPayload = JSON.stringify(
      { mcpServers: {}, security: { auth: { selectedType: "oauth-personal" } } },
      null,
      2
    );
  }

  const profile = saveProfile(name, {
    installationId,
    tokenPayload,
    settingsPayload,
  });

  if (!getActiveProfileName()) {
    setActiveProfileName(name);
    profile.isActive = true;
  }

  return profile;
}

/**
 * Deletes a profile
 */
export function deleteProfile(name: string): boolean {
  const profileDir = getProfileDir(name);
  if (!existsSync(profileDir)) {
    return false;
  }

  rmSync(profileDir, { recursive: true, force: true });

  // If deleted profile was active, clear active marker
  if (getActiveProfileName() === name) {
    try {
      writeFileSync(ACTIVE_PROFILE_FILE, "", "utf-8");
    } catch {
      // ignore
    }
  }

  return true;
}
