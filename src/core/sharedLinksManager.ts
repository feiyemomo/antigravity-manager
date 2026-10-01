import {
  existsSync,
  lstatSync,
  mkdirSync,
  readdirSync,
  readlinkSync,
  rmdirSync,
  statSync,
  symlinkSync,
  unlinkSync,
  copyFileSync,
  renameSync,
  rmSync,
} from "node:fs";
import { dirname, join, resolve } from "node:path";
import {
  SHARED_CONVERSATIONS_DIR,
  SHARED_DIR,
  SHARED_SKILLS_DIR,
  PRIMARY_CONVERSATIONS_DIR,
  IDE_CONVERSATIONS_DIR,
  PRIMARY_SKILLS_DIR,
  ANTIGRAVITY_DIR_SKILLS_DIR,
  ANTIGRAVITY_DIR,
  ANTIGRAVITY_IDE_DIR,
  ANTIGRAVITY_CONFIG_DIR,
} from "./constants.js";
import { isAntigravityRunning } from "./processGuard.js";
import type { SymlinkStatus } from "./types.js";

/**
 * Initializes the global shared directory structure
 */
export function initSharedDirectories(): void {
  if (!existsSync(SHARED_DIR)) {
    mkdirSync(SHARED_DIR, { recursive: true });
  }
  if (!existsSync(SHARED_CONVERSATIONS_DIR)) {
    mkdirSync(SHARED_CONVERSATIONS_DIR, { recursive: true });
  }
  if (!existsSync(SHARED_SKILLS_DIR)) {
    mkdirSync(SHARED_SKILLS_DIR, { recursive: true });
  }
}

/**
 * Normalizes a file path for comparison (resolves lowercase drive letters, short paths, etc.)
 */
function normalizePath(p: string): string {
  try {
    return resolve(p).toLowerCase().replace(/\\/g, "/");
  } catch {
    return p.toLowerCase().replace(/\\/g, "/");
  }
}

/**
 * Recursively copies a directory
 */
function copyDirRecursive(src: string, dest: string): void {
  if (!existsSync(dest)) {
    mkdirSync(dest, { recursive: true });
  }
  const entries = readdirSync(src, { withFileTypes: true });
  for (const entry of entries) {
    const srcPath = join(src, entry.name);
    const destPath = join(dest, entry.name);
    if (entry.isDirectory()) {
      copyDirRecursive(srcPath, destPath);
    } else {
      if (!existsSync(destPath)) {
        copyFileSync(srcPath, destPath);
      }
    }
  }
}

/**
 * Checks the status and health of a symbolic link or junction
 */
export function checkSymlinkStatus(linkPath: string, expectedTarget: string): SymlinkStatus {
  const result: SymlinkStatus = {
    path: linkPath,
    expectedTarget,
    isSymlink: false,
    targetExists: existsSync(expectedTarget),
    isHealthy: false,
  };

  if (!existsSync(linkPath) && !isSymbolicLinkOrJunction(linkPath)) {
    result.error = "Path does not exist";
    return result;
  }

  try {
    const lstat = lstatSync(linkPath);
    const isSym = lstat.isSymbolicLink();
    result.isSymlink = isSym;

    if (!isSym) {
      result.error = "Path is a regular directory, not a symbolic link";
      return result;
    }

    const actualTarget = readlinkSync(linkPath);
    result.actualTarget = actualTarget;

    const normActual = normalizePath(resolve(dirname(linkPath), actualTarget));
    const normExpected = normalizePath(expectedTarget);

    if (normActual === normExpected || normActual.endsWith(normExpected.replace(/^[a-z]:/i, ""))) {
      result.isHealthy = result.targetExists;
      if (!result.targetExists) {
        result.error = "Symlink target directory does not exist";
      }
    } else {
      result.error = `Symlink points to wrong target: ${actualTarget} (expected: ${expectedTarget})`;
    }
  } catch (err) {
    result.error = err instanceof Error ? err.message : String(err);
  }

  return result;
}

/**
 * Checks whether a path is a symbolic link or junction without throwing if target doesn't exist
 */
function isSymbolicLinkOrJunction(p: string): boolean {
  try {
    return lstatSync(p).isSymbolicLink();
  } catch {
    return false;
  }
}

/**
 * Safely removes a symlink or junction
 */
function removeSymlinkOrJunction(p: string): void {
  try {
    if (process.platform === "win32") {
      rmdirSync(p);
    } else {
      unlinkSync(p);
    }
  } catch {
    try {
      unlinkSync(p);
    } catch {
      // ignore
    }
  }
}

/**
 * Ensures a directory is linked to the shared target via symlink/junction
 */
export function ensureSharedDirectoryLinked(
  linkPath: string,
  sharedTarget: string,
  options?: { skipProcessCheck?: boolean }
): { success: boolean; migrated: boolean; created: boolean; error?: string } {
  initSharedDirectories();

  if (!existsSync(sharedTarget)) {
    mkdirSync(sharedTarget, { recursive: true });
  }

  const status = checkSymlinkStatus(linkPath, sharedTarget);
  if (status.isHealthy) {
    return { success: true, migrated: false, created: false };
  }

  // Safety: If Antigravity is running and we need to modify linkPath, refuse
  if (!options?.skipProcessCheck && existsSync(linkPath) && isAntigravityRunning()) {
    return {
      success: false,
      migrated: false,
      created: false,
      error: "Antigravity process is running. Cannot link shared directories while the app is active.",
    };
  }

  let migrated = false;

  // Case 1: linkPath is a regular directory
  if (existsSync(linkPath) && !isSymbolicLinkOrJunction(linkPath)) {
    const isDir = statSync(linkPath).isDirectory();
    if (isDir) {
      const backupPath = `${linkPath}.bak.${Date.now()}`;
      try {
        // Copy existing files to shared target
        copyDirRecursive(linkPath, sharedTarget);
        migrated = true;

        // Move current directory to backup
        renameSync(linkPath, backupPath);

        // Create symlink/junction
        if (process.platform === "win32") {
          symlinkSync(sharedTarget, linkPath, "junction");
        } else {
          symlinkSync(sharedTarget, linkPath, "dir");
        }

        // Clean up backup directory if symlink succeeded
        try {
          rmSync(backupPath, { recursive: true, force: true });
        } catch {
          // ignore cleanup failure
        }

        return { success: true, migrated: true, created: true };
      } catch (err) {
        // Rollback: restore backup if needed
        if (existsSync(backupPath) && !existsSync(linkPath)) {
          try {
            renameSync(backupPath, linkPath);
          } catch {
            // rollback failure
          }
        }
        return {
          success: false,
          migrated,
          created: false,
          error: err instanceof Error ? err.message : String(err),
        };
      }
    }
  }

  // Case 2: Broken or misplaced symlink
  if (isSymbolicLinkOrJunction(linkPath)) {
    removeSymlinkOrJunction(linkPath);
  }

  // Case 3: Link does not exist
  try {
    const parentDir = dirname(linkPath);
    if (!existsSync(parentDir)) {
      mkdirSync(parentDir, { recursive: true });
    }

    if (process.platform === "win32") {
      symlinkSync(sharedTarget, linkPath, "junction");
    } else {
      symlinkSync(sharedTarget, linkPath, "dir");
    }

    return { success: true, migrated: false, created: true };
  } catch (err) {
    return {
      success: false,
      migrated: false,
      created: false,
      error: err instanceof Error ? err.message : String(err),
    };
  }
}

/**
 * Returns health status of all tracked shared directories
 */
export function checkAllSharedLinks(): Record<string, SymlinkStatus> {
  const results: Record<string, SymlinkStatus> = {};

  results["conversations:primary"] = checkSymlinkStatus(
    PRIMARY_CONVERSATIONS_DIR,
    SHARED_CONVERSATIONS_DIR
  );

  if (existsSync(ANTIGRAVITY_IDE_DIR)) {
    results["conversations:ide"] = checkSymlinkStatus(
      IDE_CONVERSATIONS_DIR,
      SHARED_CONVERSATIONS_DIR
    );
  }

  if (existsSync(ANTIGRAVITY_CONFIG_DIR)) {
    results["skills:config"] = checkSymlinkStatus(
      PRIMARY_SKILLS_DIR,
      SHARED_SKILLS_DIR
    );
  }

  if (existsSync(ANTIGRAVITY_DIR)) {
    results["skills:antigravity"] = checkSymlinkStatus(
      ANTIGRAVITY_DIR_SKILLS_DIR,
      SHARED_SKILLS_DIR
    );
  }

  return results;
}

/**
 * Sets up and links all shared directories
 */
export function ensureAllSharedLinks(options?: { skipProcessCheck?: boolean }): {
  total: number;
  healthy: number;
  migrated: number;
  errors: string[];
} {
  initSharedDirectories();

  const errors: string[] = [];
  let healthy = 0;
  let migratedCount = 0;
  let total = 0;

  const pairs: Array<{ link: string; target: string }> = [
    { link: PRIMARY_CONVERSATIONS_DIR, target: SHARED_CONVERSATIONS_DIR },
  ];

  if (existsSync(ANTIGRAVITY_IDE_DIR)) {
    pairs.push({ link: IDE_CONVERSATIONS_DIR, target: SHARED_CONVERSATIONS_DIR });
  }

  if (existsSync(ANTIGRAVITY_CONFIG_DIR)) {
    pairs.push({ link: PRIMARY_SKILLS_DIR, target: SHARED_SKILLS_DIR });
  }

  if (existsSync(ANTIGRAVITY_DIR)) {
    pairs.push({ link: ANTIGRAVITY_DIR_SKILLS_DIR, target: SHARED_SKILLS_DIR });
  }

  total = pairs.length;

  for (const pair of pairs) {
    const res = ensureSharedDirectoryLinked(pair.link, pair.target, options);
    if (res.success) {
      healthy++;
      if (res.migrated) migratedCount++;
    } else if (res.error) {
      errors.push(`${pair.link}: ${res.error}`);
    }
  }

  return { total, healthy, migrated: migratedCount, errors };
}
