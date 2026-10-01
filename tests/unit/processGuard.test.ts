import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  findAntigravityProcesses,
  isAntigravityRunning,
  assertNoAntigravityRunning,
  ProcessSafetyError,
  processInspector,
} from "../../src/core/processGuard.js";

describe("Process Guard", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("should create ProcessSafetyError with proper message and process list", () => {
    const procs = [
      { pid: 1234, name: "Antigravity.exe" },
      { pid: 5678, name: "agy.exe" },
    ];
    const err = new ProcessSafetyError(procs, "switching accounts");
    expect(err.name).toBe("ProcessSafetyError");
    expect(err.processes).toEqual(procs);
    expect(err.message).toContain("PROCESS SAFETY VIOLATION");
    expect(err.message).toContain("Antigravity.exe (PID: 1234)");
    expect(err.message).toContain("agy.exe (PID: 5678)");
    expect(err.message).toContain("switching accounts");
  });

  it("should throw ProcessSafetyError when Antigravity process is detected", () => {
    vi.spyOn(processInspector, "listProcesses").mockReturnValue(
      `"Antigravity.exe","9999","Console","1","250,000 K"\n`
    );

    expect(() => assertNoAntigravityRunning("switching accounts")).toThrow(ProcessSafetyError);
  });

  it("should NOT throw when no target processes are running", () => {
    vi.spyOn(processInspector, "listProcesses").mockReturnValue(
      `"notepad.exe","8888","Console","1","10,000 K"\n`
    );

    expect(() => assertNoAntigravityRunning("switching accounts")).not.toThrow();
  });

  it("should ignore current process PID", () => {
    const currentPid = process.pid;
    vi.spyOn(processInspector, "listProcesses").mockReturnValue(
      `"agy-tools.exe","${currentPid}","Console","1","50,000 K"\n`
    );

    const procs = findAntigravityProcesses();
    expect(procs.some((p) => p.pid === currentPid)).toBe(false);
  });

  it("isAntigravityRunning should return boolean", () => {
    const running = isAntigravityRunning();
    expect(typeof running).toBe("boolean");
  });
});
