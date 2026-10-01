import { describe, it, expect, beforeEach, afterEach } from "vitest";
import {
  checkSymlinkStatus,
  ensureSharedDirectoryLinked,
  initSharedDirectories,
} from "../../src/core/sharedLinksManager.js";
import { existsSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";

describe("Shared Links Manager", () => {
  const testRoot = join(tmpdir(), "agy_test_shared_" + Date.now());
  const linkDir = join(testRoot, "link_folder");
  const targetDir = join(testRoot, "target_folder");

  beforeEach(() => {
    mkdirSync(testRoot, { recursive: true });
    mkdirSync(targetDir, { recursive: true });
    initSharedDirectories();
  });

  afterEach(() => {
    try {
      rmSync(testRoot, { recursive: true, force: true });
    } catch {
      // ignore
    }
  });

  it("should report non-existent link", () => {
    const status = checkSymlinkStatus(linkDir, targetDir);
    expect(status.isSymlink).toBe(false);
    expect(status.isHealthy).toBe(false);
    expect(status.error).toContain("does not exist");
  });

  it("should report regular directory if not a symlink", () => {
    mkdirSync(linkDir, { recursive: true });
    const status = checkSymlinkStatus(linkDir, targetDir);
    expect(status.isSymlink).toBe(false);
    expect(status.isHealthy).toBe(false);
    expect(status.error).toContain("regular directory");
  });

  it("should create symlink/junction and link regular directory with migration", () => {
    // Put a sample file in linkDir
    mkdirSync(linkDir, { recursive: true });
    writeFileSync(join(linkDir, "sample.txt"), "hello world", "utf-8");

    const result = ensureSharedDirectoryLinked(linkDir, targetDir, { skipProcessCheck: true });
    expect(result.success).toBe(true);
    expect(result.created).toBe(true);

    // Verify file was migrated to targetDir
    expect(existsSync(join(targetDir, "sample.txt"))).toBe(true);

    // Verify link status is healthy
    const status = checkSymlinkStatus(linkDir, targetDir);
    expect(status.isSymlink).toBe(true);
    expect(status.isHealthy).toBe(true);
  });
});
