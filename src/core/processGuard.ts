import { execSync } from "node:child_process";
import { TARGET_PROCESS_NAMES } from "./constants.js";
import type { RunningProcess } from "./types.js";

export class ProcessSafetyError extends Error {
  public processes: RunningProcess[];

  constructor(processes: RunningProcess[], action: string = "switching accounts") {
    const procList = processes.map((p) => `${p.name} (PID: ${p.pid})`).join(", ");
    const message =
      `[PROCESS SAFETY VIOLATION] Cannot perform ${action} while Antigravity is running.\n` +
      `Detected active process(es): ${procList}\n` +
      `Process Safety Policy: Overwriting credentials or isolated files while Antigravity IDE/CLI is running is blocked by default to prevent file-lock or credential corruption.\n\n` +
      `Available Solutions:\n` +
      `  1. (Safe Recommended): Exit Antigravity IDE, then run the switch command.\n` +
      `  2. (Live Hot-Swap): Force an online hot-swap without closing Antigravity:\n` +
      `     agy-tools switch <name> --live\n` +
      `     (Add '--restart-lsp' to reload background language_server instantly)`;
    super(message);
    this.name = "ProcessSafetyError";
    this.processes = processes;
  }
}

/**
 * Gracefully terminates the background language_server child process.
 * Antigravity IDE will automatically respawn it with fresh credentials without closing the IDE window.
 */
export function restartLanguageServer(): boolean {
  try {
    if (process.platform === "win32") {
      execSync("taskkill /F /IM language_server.exe", {
        stdio: "ignore",
        windowsHide: true,
      });
    } else {
      execSync("pkill -9 -f language_server", {
        stdio: "ignore",
      });
    }
    return true;
  } catch {
    return false;
  }
}

export const processInspector = {
  listProcesses(): string {
    try {
      if (process.platform === "win32") {
        return execSync("tasklist /FO CSV /NH", {
          encoding: "utf-8",
          stdio: ["ignore", "pipe", "ignore"],
          windowsHide: true,
        });
      } else {
        return execSync("ps -eo pid,comm", {
          encoding: "utf-8",
          stdio: ["ignore", "pipe", "ignore"],
        });
      }
    } catch {
      return "";
    }
  },
};

/**
 * Scans the operating system for running Antigravity IDE or CLI processes.
 * Excludes the current Node.js process.
 */
export function findAntigravityProcesses(): RunningProcess[] {
  const currentPid = process.pid;
  const running: RunningProcess[] = [];

  try {
    const stdout = processInspector.listProcesses();
    if (!stdout) return [];

    if (process.platform === "win32") {
      const lines = stdout.split(/\r?\n/);
      for (const line of lines) {
        if (!line.trim()) continue;
        // Parse CSV columns: "ImageName","PID","SessionName","Session#","MemUsage"
        const parts = line.split('","').map((s) => s.replace(/^"|"$/g, ""));
        if (parts.length >= 2) {
          const imageName = parts[0];
          const pid = parseInt(parts[1], 10);

          if (isNaN(pid) || pid === currentPid) continue;

          const lowerName = imageName.toLowerCase();
          const matches = TARGET_PROCESS_NAMES.some((target) =>
            lowerName === target.toLowerCase() ||
            lowerName === `${target.toLowerCase()}.exe`
          );

          if (matches) {
            running.push({ pid, name: imageName });
          }
        }
      }
    } else {
      // Unix / macOS / Linux
      const lines = stdout.split(/\r?\n/);
      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed) continue;
        const [pidStr, ...commParts] = trimmed.split(/\s+/);
        const pid = parseInt(pidStr, 10);
        const comm = commParts.join(" ");

        if (isNaN(pid) || pid === currentPid) continue;

        const lowerComm = comm.toLowerCase();
        const matches = TARGET_PROCESS_NAMES.some((target) =>
          lowerComm === target.toLowerCase() ||
          lowerComm.endsWith(`/${target.toLowerCase()}`)
        );

        if (matches) {
          running.push({ pid, name: comm });
        }
      }
    }
  } catch {
    // If process inspection fails, fail safe by returning empty list
  }

  return running;
}

/**
 * Returns true if any Antigravity IDE or CLI processes are running.
 */
export function isAntigravityRunning(): boolean {
  return findAntigravityProcesses().length > 0;
}

/**
 * Asserts that no Antigravity process is running.
 * Throws ProcessSafetyError if any process is found.
 */
export function assertNoAntigravityRunning(action: string = "switching accounts"): void {
  const activeProcesses = findAntigravityProcesses();
  if (activeProcesses.length > 0) {
    throw new ProcessSafetyError(activeProcesses, action);
  }
}
