import {
  existsSync,
  mkdirSync,
  readFileSync,
  renameSync,
  unlinkSync,
  writeFileSync,
} from "node:fs";
import { dirname } from "node:path";
import {
  CREDENTIAL_TARGET,
  CREDENTIAL_USERNAME,
  IDE_INSTALLATION_ID_FILE,
  PRIMARY_INSTALLATION_ID_FILE,
  PRIMARY_SETTINGS_FILE,
  PRIMARY_TOKEN_FILE,
  SECONDARY_TOKEN_FILE,
  ANTIGRAVITY_DIR_TOKEN_FILE,
  ANTIGRAVITY_DIR_SETTINGS_FILE,
  ANTIGRAVITY_DIR,
  ANTIGRAVITY_IDE_DIR,
} from "./constants.js";
import { assertNoAntigravityRunning } from "./processGuard.js";
import { credentialManager } from "./credentialManager.js";
import {
  getActiveProfileName,
  getProfile,
  listProfiles,
  saveProfile,
  setActiveProfileName,
} from "./profileManager.js";
import { ensureAllSharedLinks } from "./sharedLinksManager.js";
import { syncLiveAntigravityUi } from "./liveUiSync.js";
import type { IsolatedFilesPayload, SwitchResult } from "./types.js";

/**
 * Safely writes a file atomically using a temporary file in the same directory
 */
function atomicWriteFile(filePath: string, content: string): void {
  const dir = dirname(filePath);
  if (!existsSync(dir)) {
    mkdirSync(dir, { recursive: true });
  }

  const tmpPath = `${filePath}.tmp.${Date.now()}.${Math.random().toString(36).substring(2, 8)}`;
  writeFileSync(tmpPath, content, "utf-8");

  try {
    renameSync(tmpPath, filePath);
  } catch {
    // On some Windows environments if destination exists, renameSync might fail
    try {
      if (existsSync(filePath)) {
        unlinkSync(filePath);
      }
      renameSync(tmpPath, filePath);
    } catch {
      writeFileSync(filePath, content, "utf-8");
      try {
        unlinkSync(tmpPath);
      } catch {
        // ignore
      }
    }
  }
}

/**
 * Finds a profile by name, email, or account ID prefix
 */
export function resolveProfile(query: string): { name: string; payload: IsolatedFilesPayload } | null {
  const q = query.trim().toLowerCase();

  // 1. Direct name match
  const direct = getProfile(query.trim());
  if (direct) {
    return { name: query.trim(), payload: direct };
  }

  // 2. Search all profiles
  const profiles = listProfiles();

  // By exact name (case-insensitive)
  const byName = profiles.find((p) => p.name.toLowerCase() === q);
  if (byName) {
    const payload = getProfile(byName.name);
    if (payload) return { name: byName.name, payload };
  }

  // By email
  const byEmail = profiles.find((p) => p.email && p.email.toLowerCase() === q);
  if (byEmail) {
    const payload = getProfile(byEmail.name);
    if (payload) return { name: byEmail.name, payload };
  }

  // By accountId prefix
  const byId = profiles.find((p) => p.accountId && p.accountId.toLowerCase().startsWith(q));
  if (byId) {
    const payload = getProfile(byId.name);
    if (payload) return { name: byId.name, payload };
  }

  return null;
}

/**
 * Synchronizes the current active Antigravity files & credentials back into the active profile folder
 */
export async function syncCurrentStateToActiveProfile(): Promise<void> {
  const activeName = getActiveProfileName();
  if (!activeName) return;

  const existingProfile = getProfile(activeName);
  if (!existingProfile) return;

  let installationId = existingProfile.installationId;
  if (existsSync(PRIMARY_INSTALLATION_ID_FILE)) {
    try {
      installationId = readFileSync(PRIMARY_INSTALLATION_ID_FILE, "utf-8").trim();
    } catch {
      // ignore
    }
  }

  let tokenPayload = existingProfile.tokenPayload;
  try {
    const credSecret = await credentialManager.read();
    if (credSecret) {
      tokenPayload = credSecret;
    } else if (existsSync(PRIMARY_TOKEN_FILE)) {
      tokenPayload = readFileSync(PRIMARY_TOKEN_FILE, "utf-8");
    }
  } catch {
    // ignore
  }

  let settingsPayload = existingProfile.settingsPayload;
  if (existsSync(PRIMARY_SETTINGS_FILE)) {
    try {
      settingsPayload = readFileSync(PRIMARY_SETTINGS_FILE, "utf-8");
    } catch {
      // ignore
    }
  }

  saveProfile(activeName, {
    installationId,
    tokenPayload,
    settingsPayload,
    metadata: {
      ...existingProfile.metadata,
      lastUsedAt: Date.now(),
    },
  });
}

/**
 * Atomically switches to a target profile.
 *
 * Sequence:
 * 1. Process safety check: refuses if Antigravity or agy is running.
 * 2. Pre-flight check: ensures target profile exists and is valid.
 * 3. Saves current active state to active profile directory.
 * 4. Takes snapshot of current active files and credentials for rollback.
 * 5. Replaces isolated files atomically.
 * 6. Writes credential to Credential Manager.
 * 7. Updates active_profile marker.
 * 8. Ensures shared symlinks (conversations, skills) are connected.
 * 9. If any failure occurs, executes full rollback to previous state.
 */
export async function switchProfile(
  targetQuery: string,
  options?: { skipProcessCheck?: boolean }
): Promise<SwitchResult> {
  // 1. Process Safety Check
  if (!options?.skipProcessCheck) {
    assertNoAntigravityRunning(`switching profile to '${targetQuery}'`);
  }

  // 2. Pre-flight check
  const resolved = resolveProfile(targetQuery);
  if (!resolved) {
    throw new Error(
      `Profile not found matching '${targetQuery}'. Use 'agy-tools profile list' to see available profiles.`
    );
  }

  const { name: targetName, payload: targetPayload } = resolved;
  const previousActiveName = getActiveProfileName();

  // If already active, still ensure files & credentials match
  if (previousActiveName && previousActiveName === targetName) {
    // Same profile, verify files and credentials
  }

  // 3. Sync current active state back to previous active profile
  if (previousActiveName && previousActiveName !== targetName) {
    try {
      await syncCurrentStateToActiveProfile();
    } catch {
      // Non-fatal if sync back fails
    }
  }

  // 4. Capture current state for rollback
  const rollbackSnapshots: Record<string, string | null> = {};
  const filesToTrack = [
    PRIMARY_INSTALLATION_ID_FILE,
    PRIMARY_TOKEN_FILE,
    SECONDARY_TOKEN_FILE,
    ANTIGRAVITY_DIR_TOKEN_FILE,
    PRIMARY_SETTINGS_FILE,
  ];

  if (existsSync(ANTIGRAVITY_IDE_DIR)) {
    filesToTrack.push(IDE_INSTALLATION_ID_FILE);
  }
  if (existsSync(ANTIGRAVITY_DIR)) {
    filesToTrack.push(ANTIGRAVITY_DIR_SETTINGS_FILE);
  }

  for (const f of filesToTrack) {
    if (existsSync(f)) {
      try {
        rollbackSnapshots[f] = readFileSync(f, "utf-8");
      } catch {
        rollbackSnapshots[f] = null;
      }
    } else {
      rollbackSnapshots[f] = null;
    }
  }

  let previousCredentialSecret: string | null = null;
  try {
    previousCredentialSecret = await credentialManager.read();
  } catch {
    previousCredentialSecret = null;
  }

  const previousActiveProfileMarker = previousActiveName;

  // Rollback actions list
  const rollbackActions: Array<() => Promise<void> | void> = [];

  try {
    // 5. Atomic File Replacements
    // 5a. Installation ID
    const installIdContent = targetPayload.installationId.trim();
    atomicWriteFile(PRIMARY_INSTALLATION_ID_FILE, installIdContent);
    rollbackActions.push(() => {
      const orig = rollbackSnapshots[PRIMARY_INSTALLATION_ID_FILE];
      if (orig !== null) writeFileSync(PRIMARY_INSTALLATION_ID_FILE, orig, "utf-8");
      else if (existsSync(PRIMARY_INSTALLATION_ID_FILE)) unlinkSync(PRIMARY_INSTALLATION_ID_FILE);
    });

    if (existsSync(ANTIGRAVITY_IDE_DIR)) {
      atomicWriteFile(IDE_INSTALLATION_ID_FILE, installIdContent);
      rollbackActions.push(() => {
        const orig = rollbackSnapshots[IDE_INSTALLATION_ID_FILE];
        if (orig !== null) writeFileSync(IDE_INSTALLATION_ID_FILE, orig, "utf-8");
        else if (existsSync(IDE_INSTALLATION_ID_FILE)) unlinkSync(IDE_INSTALLATION_ID_FILE);
      });
    }

    // 5b. Token files (both jetski-standalone-oauth-token and antigravity-oauth-token)
    const tokenContent = targetPayload.tokenPayload.trim();
    atomicWriteFile(PRIMARY_TOKEN_FILE, tokenContent);
    rollbackActions.push(() => {
      const orig = rollbackSnapshots[PRIMARY_TOKEN_FILE];
      if (orig !== null) writeFileSync(PRIMARY_TOKEN_FILE, orig, "utf-8");
      else if (existsSync(PRIMARY_TOKEN_FILE)) unlinkSync(PRIMARY_TOKEN_FILE);
    });

    atomicWriteFile(SECONDARY_TOKEN_FILE, tokenContent);
    rollbackActions.push(() => {
      const orig = rollbackSnapshots[SECONDARY_TOKEN_FILE];
      if (orig !== null) writeFileSync(SECONDARY_TOKEN_FILE, orig, "utf-8");
      else if (existsSync(SECONDARY_TOKEN_FILE)) unlinkSync(SECONDARY_TOKEN_FILE);
    });

    if (existsSync(ANTIGRAVITY_DIR)) {
      atomicWriteFile(ANTIGRAVITY_DIR_TOKEN_FILE, tokenContent);
      rollbackActions.push(() => {
        const orig = rollbackSnapshots[ANTIGRAVITY_DIR_TOKEN_FILE];
        if (orig !== null) writeFileSync(ANTIGRAVITY_DIR_TOKEN_FILE, orig, "utf-8");
        else if (existsSync(ANTIGRAVITY_DIR_TOKEN_FILE)) unlinkSync(ANTIGRAVITY_DIR_TOKEN_FILE);
      });
    }

    // 5c. Settings files
    const settingsContent = targetPayload.settingsPayload.trim();
    atomicWriteFile(PRIMARY_SETTINGS_FILE, settingsContent);
    rollbackActions.push(() => {
      const orig = rollbackSnapshots[PRIMARY_SETTINGS_FILE];
      if (orig !== null) writeFileSync(PRIMARY_SETTINGS_FILE, orig, "utf-8");
      else if (existsSync(PRIMARY_SETTINGS_FILE)) unlinkSync(PRIMARY_SETTINGS_FILE);
    });

    if (existsSync(ANTIGRAVITY_DIR)) {
      atomicWriteFile(ANTIGRAVITY_DIR_SETTINGS_FILE, settingsContent);
      rollbackActions.push(() => {
        const orig = rollbackSnapshots[ANTIGRAVITY_DIR_SETTINGS_FILE];
        if (orig !== null) writeFileSync(ANTIGRAVITY_DIR_SETTINGS_FILE, orig, "utf-8");
        else if (existsSync(ANTIGRAVITY_DIR_SETTINGS_FILE)) unlinkSync(ANTIGRAVITY_DIR_SETTINGS_FILE);
      });
    }

    // 6. Credential Manager Write
    const credWriteSuccess = await credentialManager.write(
      CREDENTIAL_TARGET,
      CREDENTIAL_USERNAME,
      tokenContent
    );

    if (!credWriteSuccess) {
      throw new Error(`Failed to write credential to Credential Manager (${CREDENTIAL_TARGET})`);
    }

    rollbackActions.push(async () => {
      if (previousCredentialSecret !== null) {
        await credentialManager.write(CREDENTIAL_TARGET, CREDENTIAL_USERNAME, previousCredentialSecret);
      } else {
        await credentialManager.delete(CREDENTIAL_TARGET);
      }
    });

    // 7. Update active_profile marker
    setActiveProfileName(targetName);
    rollbackActions.push(() => {
      if (previousActiveProfileMarker) {
        setActiveProfileName(previousActiveProfileMarker);
      }
    });

    // 8. Ensure shared symlinks are healthy
    try {
      ensureAllSharedLinks();
    } catch {
      // Non-fatal if symlink migration has warnings
    }

    // 9. Live UI sync for Antigravity IDE (app_storage.json, Chrome DevTools hot-push & quota sync)
    if (targetPayload.metadata?.email) {
      try {
        let accessToken: string | undefined;
        try {
          const parsed = JSON.parse(targetPayload.tokenPayload);
          accessToken = parsed.token?.access_token || parsed.access_token || parsed.token;
        } catch {
          // ignore
        }
        await syncLiveAntigravityUi(
          targetPayload.metadata.email,
          targetPayload.metadata.name,
          accessToken
        );
      } catch {
        // Non-fatal if Antigravity is not currently running
      }
    }

    // 10. Update lastUsedAt in profile
    saveProfile(targetName, {
      ...targetPayload,
      metadata: {
        ...targetPayload.metadata,
        lastUsedAt: Date.now(),
      },
    });

    return {
      success: true,
      profileName: targetName,
      email: targetPayload.metadata?.email,
      previousProfileName: previousActiveName || undefined,
    };
  } catch (err) {
    // ATOMIC ROLLBACK
    for (let i = rollbackActions.length - 1; i >= 0; i--) {
      try {
        const action = rollbackActions[i];
        await action();
      } catch {
        // Continue rolling back remaining items
      }
    }

    throw new Error(
      `[ATOMIC SWITCH FAILED & ROLLED BACK]: ${err instanceof Error ? err.message : String(err)}`
    );
  }
}
