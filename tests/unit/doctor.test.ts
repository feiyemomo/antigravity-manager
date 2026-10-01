import { describe, it, expect, vi, beforeEach } from "vitest";
import { runDiagnostics } from "../../src/core/doctorService.js";
import * as processGuard from "../../src/core/processGuard.js";
import { credentialManager } from "../../src/core/credentialManager.js";

describe("Doctor Service Diagnostics", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("should generate full diagnostic report with all 4 check categories", async () => {
    vi.spyOn(processGuard, "findAntigravityProcesses").mockReturnValue([]);
    vi.spyOn(credentialManager, "isAvailable").mockResolvedValue(true);
    vi.spyOn(credentialManager, "read").mockResolvedValue(
      JSON.stringify({
        token: {
          access_token: "ya29.test",
          expiry: new Date(Date.now() + 3600000).toISOString(),
        },
        email: "test@example.com",
      })
    );

    const report = await runDiagnostics();

    expect(report).toBeDefined();
    expect(report.checks.processes).toBeDefined();
    expect(report.checks.credentials).toBeDefined();
    expect(report.checks.profiles).toBeDefined();
    expect(report.checks.symlinks).toBeDefined();

    expect(report.checks.processes.status).toBe("ok");
    expect(report.checks.credentials.status).toBe("ok");
  });

  it("should flag warn on processes if Antigravity is running", async () => {
    vi.spyOn(processGuard, "findAntigravityProcesses").mockReturnValue([
      { pid: 1234, name: "Antigravity.exe" },
    ]);

    const report = await runDiagnostics();
    expect(report.checks.processes.status).toBe("warn");
    expect(report.checks.processes.message).toContain("running");
    expect(report.overallStatus).not.toBe("healthy");
  });

  it("should flag error on credentials if Credential Manager is unavailable", async () => {
    vi.spyOn(processGuard, "findAntigravityProcesses").mockReturnValue([]);
    vi.spyOn(credentialManager, "isAvailable").mockResolvedValue(false);

    const report = await runDiagnostics();
    expect(report.checks.credentials.status).toBe("error");
    expect(report.overallStatus).toBe("critical");
  });
});
