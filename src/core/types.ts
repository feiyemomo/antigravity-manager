export interface RunningProcess {
  pid: number;
  name: string;
  commandLine?: string;
}

export interface ProfileMetadata {
  name: string;
  email?: string;
  accountId?: string;
  createdAt: string;
  updatedAt: string;
  tier?: string;
  lastUsedAt?: number;
}

export interface ProfileInfo extends ProfileMetadata {
  isActive: boolean;
  hasInstallationId: boolean;
  hasToken: boolean;
  hasSettings: boolean;
  tokenExpiresAt?: number;
  isTokenExpired?: boolean;
}

export interface IsolatedFilesPayload {
  installationId: string;
  tokenPayload: string; // JSON string
  settingsPayload: string; // JSON string
  metadata?: Partial<ProfileMetadata>;
}

export interface SwitchResult {
  success: boolean;
  profileName: string;
  email?: string;
  previousProfileName?: string;
  revertedOnError?: boolean;
  error?: string;
}

export interface SymlinkStatus {
  path: string;
  expectedTarget: string;
  actualTarget?: string;
  isSymlink: boolean;
  targetExists: boolean;
  isHealthy: boolean;
  error?: string;
}

export interface DoctorCheckItem {
  name: string;
  status: "ok" | "warn" | "error";
  message: string;
  details?: string[];
  remediation?: string;
}

export interface DoctorReport {
  timestamp: string;
  overallStatus: "healthy" | "degraded" | "critical";
  checks: {
    processes: DoctorCheckItem;
    credentials: DoctorCheckItem;
    profiles: DoctorCheckItem;
    symlinks: DoctorCheckItem;
  };
  summary: string[];
}
