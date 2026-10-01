import { homedir } from "node:os";
import { join } from "node:path";

// ============================================
// Multi-Account Profile & Shared Paths
// ============================================

export const AGY_AUTH_DIR = join(homedir(), ".agy_auth");
export const PROFILES_DIR = join(AGY_AUTH_DIR, "profiles");
export const SHARED_DIR = join(AGY_AUTH_DIR, "shared");
export const SHARED_CONVERSATIONS_DIR = join(SHARED_DIR, "conversations");
export const SHARED_SKILLS_DIR = join(SHARED_DIR, "skills");
export const ACTIVE_PROFILE_FILE = join(AGY_AUTH_DIR, "active_profile");
export const PROFILES_METADATA_FILE = join(AGY_AUTH_DIR, "metadata.json");

// ============================================
// Target Antigravity Files (Isolated / Swapped)
// ============================================

export const GEMINI_BASE_DIR = join(homedir(), ".gemini");
export const ANTIGRAVITY_DIR = join(GEMINI_BASE_DIR, "antigravity");
export const ANTIGRAVITY_IDE_DIR = join(GEMINI_BASE_DIR, "antigravity-ide");
export const ANTIGRAVITY_CONFIG_DIR = join(GEMINI_BASE_DIR, "config");

export const PRIMARY_INSTALLATION_ID_FILE = join(ANTIGRAVITY_DIR, "installation_id");
export const IDE_INSTALLATION_ID_FILE = join(ANTIGRAVITY_IDE_DIR, "installation_id");

export const PRIMARY_TOKEN_FILE = join(GEMINI_BASE_DIR, "jetski-standalone-oauth-token");
export const SECONDARY_TOKEN_FILE = join(GEMINI_BASE_DIR, "antigravity-oauth-token");
export const ANTIGRAVITY_DIR_TOKEN_FILE = join(ANTIGRAVITY_DIR, "antigravity-oauth-token");

export const PRIMARY_SETTINGS_FILE = join(GEMINI_BASE_DIR, "settings.json");
export const ANTIGRAVITY_DIR_SETTINGS_FILE = join(ANTIGRAVITY_DIR, "settings.json");

function getElectronUserDataDir(): string {
  if (process.platform === "win32") {
    return join(process.env.APPDATA || join(homedir(), "AppData", "Roaming"), "Antigravity");
  } else if (process.platform === "darwin") {
    return join(homedir(), "Library", "Application Support", "Antigravity");
  } else {
    return join(homedir(), ".config", "Antigravity");
  }
}

export const ANTIGRAVITY_ELECTRON_USER_DATA_DIR = getElectronUserDataDir();
export const ANTIGRAVITY_APP_STORAGE_FILE = join(ANTIGRAVITY_ELECTRON_USER_DATA_DIR, "app_storage.json");
export const ANTIGRAVITY_DEVTOOLS_PORT_FILE = join(ANTIGRAVITY_ELECTRON_USER_DATA_DIR, "DevToolsActivePort");

// ============================================
// Target Shared Directories (Symlinked)
// ============================================

export const PRIMARY_CONVERSATIONS_DIR = join(ANTIGRAVITY_DIR, "conversations");
export const IDE_CONVERSATIONS_DIR = join(ANTIGRAVITY_IDE_DIR, "conversations");

export const PRIMARY_SKILLS_DIR = join(ANTIGRAVITY_CONFIG_DIR, "skills");
export const ANTIGRAVITY_DIR_SKILLS_DIR = join(ANTIGRAVITY_DIR, "skills");

// ============================================
// Credential Store Configuration
// ============================================

export const CREDENTIAL_TARGET = "gemini:antigravity";
export const CREDENTIAL_USERNAME = "antigravity";

// ============================================
// Monitored Process Names
// ============================================

export const TARGET_PROCESS_NAMES = [
  "antigravity.exe",
  "antigravity",
  "agy.exe",
  "agy",
  "antigravity-ide.exe",
  "antigravity-ide",
] as const;
