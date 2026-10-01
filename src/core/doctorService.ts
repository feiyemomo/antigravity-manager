import { existsSync } from "node:fs";
import {
  CREDENTIAL_TARGET,
  PRIMARY_INSTALLATION_ID_FILE,
  PRIMARY_SETTINGS_FILE,
  PRIMARY_TOKEN_FILE,
  PROFILES_DIR,
} from "./constants.js";
import { credentialManager } from "./credentialManager.js";
import { findAntigravityProcesses } from "./processGuard.js";
import {
  captureCurrentAsProfile,
  extractEmailFromTokenPayload,
  extractExpiryFromTokenPayload,
  getActiveProfileName,
  listProfiles,
} from "./profileManager.js";
import {
  checkAllSharedLinks,
  ensureAllSharedLinks,
  initSharedDirectories,
} from "./sharedLinksManager.js";
import type { DoctorCheckItem, DoctorReport } from "./types.js";

/**
 * Runs a comprehensive self-diagnostic check
 */
export async function runDiagnostics(): Promise<DoctorReport> {
  const timestamp = new Date().toISOString();
  const summary: string[] = [];

  // ============================================
  // 1. Process Safety Check
  // ============================================
  const runningProcs = findAntigravityProcesses();
  let processCheck: DoctorCheckItem;

  if (runningProcs.length > 0) {
    const procDetails = runningProcs.map((p) => `${p.name} (PID: ${p.pid})`);
    processCheck = {
      name: "Process Safety",
      status: "warn",
      message: `Antigravity process is currently running (${runningProcs.length} active process(es))`,
      details: procDetails,
      remediation: "Close Antigravity IDE and CLI before executing account switching.",
    };
    summary.push(`[WARN] ${runningProcs.length} Antigravity process(es) running. Account switching is locked.`);
  } else {
    processCheck = {
      name: "Process Safety",
      status: "ok",
      message: "No active Antigravity IDE or CLI processes detected (Safe for switching)",
      details: ["System is ready for atomic profile switching."],
    };
    summary.push("[OK] Process safety check passed. No Antigravity instances active.");
  }

  // ============================================
  // 2. Credential Manager Check
  // ============================================
  let credentialCheck: DoctorCheckItem;
  const isCredAvailable = await credentialManager.isAvailable();

  if (!isCredAvailable) {
    credentialCheck = {
      name: "Credential Storage",
      status: "error",
      message: "System Credential Manager is not accessible",
      details: [
        process.platform === "win32"
          ? "Windows Credential Manager / advapi32.dll / cmdkey failed to respond."
          : "System keychain utility is missing.",
      ],
      remediation: "Check system permissions and ensure Credential Manager service is running.",
    };
    summary.push("[ERROR] System Credential Manager is unavailable.");
  } else {
    const credSecret = await credentialManager.read(CREDENTIAL_TARGET);
    if (!credSecret) {
      credentialCheck = {
        name: "Credential Storage",
        status: "warn",
        message: `Credential '${CREDENTIAL_TARGET}' not found in Credential Manager`,
        details: [
          `Target: ${CREDENTIAL_TARGET}`,
          "Antigravity might not be logged in, or credentials have not been stored yet.",
        ],
        remediation: "Run 'agy-tools login' or 'agy-tools profile save <name>' to record credentials.",
      };
      summary.push(`[WARN] Credential '${CREDENTIAL_TARGET}' not present in Credential Manager.`);
    } else {
      let email: string | undefined;
      let expiresAt: number | undefined;
      let isExpired = false;

      try {
        const parsed = JSON.parse(credSecret);
        email = extractEmailFromTokenPayload(parsed);
        expiresAt = extractExpiryFromTokenPayload(parsed);
        if (expiresAt && expiresAt < Date.now()) {
          isExpired = true;
        }
      } catch {
        // ignore parse error
      }

      const details = [
        `Target: ${CREDENTIAL_TARGET} (Readable)`,
        `Account: ${email || "unknown"}`,
        expiresAt
          ? `Token Expiry: ${new Date(expiresAt).toLocaleString()} ${isExpired ? "[EXPIRED]" : "[VALID]"}`
          : "Token Expiry: unknown",
      ];

      credentialCheck = {
        name: "Credential Storage",
        status: isExpired ? "warn" : "ok",
        message: isExpired
          ? `Credential '${CREDENTIAL_TARGET}' is expired`
          : `Credential '${CREDENTIAL_TARGET}' is accessible and valid`,
        details,
        remediation: isExpired ? "Run 'agy-tools accounts refresh' to renew expired tokens." : undefined,
      };
      summary.push(
        isExpired
          ? `[WARN] Credential '${CREDENTIAL_TARGET}' has expired.`
          : `[OK] Credential storage accessible for '${CREDENTIAL_TARGET}' (${email || "valid"}).`
      );
    }
  }

  // ============================================
  // 3. Profiles Storage Check
  // ============================================
  let profilesCheck: DoctorCheckItem;
  const profiles = listProfiles();
  const activeProfile = getActiveProfileName();

  const profileDetails: string[] = [
    `Profiles directory: ${PROFILES_DIR} (${profiles.length} profile(s))`,
    `Active Profile marker: ${activeProfile ? `'${activeProfile}'` : "none set"}`,
  ];

  let profilesStatus: "ok" | "warn" | "error" = "ok";

  if (profiles.length === 0) {
    profilesStatus = "warn";
    profileDetails.push("No multi-account profiles created yet.");
    summary.push("[WARN] No profiles configured in ~/.agy_auth/profiles.");
  } else {
    for (const p of profiles) {
      const activeTag = p.isActive ? " [ACTIVE]" : "";
      const health = p.hasInstallationId && p.hasToken && p.hasSettings ? "healthy" : "incomplete";
      profileDetails.push(
        `  - ${p.name}${activeTag}: ${p.email || "no-email"} (${health}, token: ${
          p.isTokenExpired ? "expired" : "valid"
        })`
      );
    }
    summary.push(`[OK] ${profiles.length} profile(s) found. Active: ${activeProfile || "none"}.`);
  }

  // Check active file consistency
  if (activeProfile) {
    const activeData = profiles.find((p) => p.name === activeProfile);
    if (!activeData) {
      profilesStatus = "error";
      profileDetails.push(`[MISMATCH] Active profile '${activeProfile}' does not exist in profiles directory!`);
    } else {
      const hasActiveFiles =
        existsSync(PRIMARY_INSTALLATION_ID_FILE) &&
        existsSync(PRIMARY_TOKEN_FILE) &&
        existsSync(PRIMARY_SETTINGS_FILE);

      if (!hasActiveFiles) {
        profilesStatus = "warn";
        profileDetails.push("[NOTICE] Some active Antigravity files are missing in ~/.gemini.");
      }
    }
  }

  profilesCheck = {
    name: "Profiles Storage",
    status: profilesStatus,
    message:
      profiles.length === 0
        ? "No multi-account profiles configured"
        : `${profiles.length} profile(s) loaded (Active: ${activeProfile || "none"})`,
    details: profileDetails,
    remediation:
      profiles.length === 0
        ? "Run 'agy-tools profile save default' to save the current account as a profile."
        : undefined,
  };

  // ============================================
  // 4. Symbolic Links Check
  // ============================================
  let symlinksCheck: DoctorCheckItem;
  const symlinkStatuses = checkAllSharedLinks();
  const symlinkDetails: string[] = [];
  let symlinkHealthyCount = 0;
  let symlinkTotalCount = 0;
  let hasRegularDir = false;

  for (const [key, s] of Object.entries(symlinkStatuses)) {
    symlinkTotalCount++;
    if (s.isHealthy) {
      symlinkHealthyCount++;
      symlinkDetails.push(`  [OK] ${key}: ${s.path} -> ${s.expectedTarget}`);
    } else {
      if (s.error && s.error.includes("regular directory")) {
        hasRegularDir = true;
        symlinkDetails.push(`  [DIR] ${key}: ${s.path} (Regular directory, not yet linked)`);
      } else {
        symlinkDetails.push(`  [FAIL] ${key}: ${s.path} (${s.error || "unhealthy"})`);
      }
    }
  }

  let symlinksStatus: "ok" | "warn" | "error" = "ok";
  let symlinksMessage = `All ${symlinkTotalCount} shared symlinks are healthy`;

  if (symlinkHealthyCount < symlinkTotalCount) {
    if (hasRegularDir) {
      symlinksStatus = "warn";
      symlinksMessage = `${symlinkHealthyCount}/${symlinkTotalCount} symlinks established. Existing directories ready to be linked.`;
      summary.push("[WARN] Shared directories (conversations/skills) not yet linked to global shared store.");
    } else {
      symlinksStatus = "error";
      symlinksMessage = `${symlinkHealthyCount}/${symlinkTotalCount} symlinks healthy. Some links are broken.`;
      summary.push("[ERROR] One or more shared symbolic links are broken.");
    }
  } else {
    summary.push(`[OK] All ${symlinkTotalCount} shared directory symlinks are healthy.`);
  }

  symlinksCheck = {
    name: "Shared Symlinks",
    status: symlinksStatus,
    message: symlinksMessage,
    details: symlinkDetails,
    remediation:
      symlinkHealthyCount < symlinkTotalCount
        ? "Run 'agy-tools doctor --fix' (with Antigravity closed) to link shared directories."
        : undefined,
  };

  // ============================================
  // Overall Status
  // ============================================
  let overallStatus: "healthy" | "degraded" | "critical" = "healthy";
  const allChecks = [processCheck, credentialCheck, profilesCheck, symlinksCheck];

  if (allChecks.some((c) => c.status === "error")) {
    overallStatus = "critical";
  } else if (allChecks.some((c) => c.status === "warn")) {
    overallStatus = "degraded";
  }

  return {
    timestamp,
    overallStatus,
    checks: {
      processes: processCheck,
      credentials: credentialCheck,
      profiles: profilesCheck,
      symlinks: symlinksCheck,
    },
    summary,
  };
}

/**
 * Automatically fixes detectable configuration and symlink issues
 */
export async function fixDoctorIssues(): Promise<{
  fixed: string[];
  failed: string[];
}> {
  const fixed: string[] = [];
  const failed: string[] = [];

  initSharedDirectories();

  // 1. If no profiles exist, try capturing current Antigravity environment as 'default'
  const profiles = listProfiles();
  if (profiles.length === 0) {
    try {
      if (existsSync(PRIMARY_TOKEN_FILE) || (await credentialManager.read())) {
        const captured = await captureCurrentAsProfile("default");
        fixed.push(`Captured active Antigravity credentials into profile 'default' (${captured.email || "OK"})`);
      }
    } catch (err) {
      failed.push(`Could not capture active profile: ${err instanceof Error ? err.message : String(err)}`);
    }
  }

  // 2. Link shared directories
  const linkResult = ensureAllSharedLinks();
  if (linkResult.errors.length > 0) {
    for (const e of linkResult.errors) {
      failed.push(`Symlink error: ${e}`);
    }
  }
  if (linkResult.migrated > 0) {
    fixed.push(`Migrated and linked ${linkResult.migrated} directory to shared global store`);
  }
  if (linkResult.healthy === linkResult.total) {
    fixed.push(`All ${linkResult.total} shared directories are linked`);
  }

  return { fixed, failed };
}
