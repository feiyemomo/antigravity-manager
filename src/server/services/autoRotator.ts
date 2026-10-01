import { loadConfig } from "../../shared/config.js";
import { logger } from "../../shared/logger.js";
import { tokenStore } from "./tokenStore.js";

class AutoRotator {
  private timer: NodeJS.Timeout | null = null;
  private isRotating = false;

  start(): void {
    this.stop();
    const config = loadConfig();
    const intervalMin = config.proxy.autoRotateIntervalMinutes || 60;
    const isEnabled = config.proxy.autoRotateAntigravity ?? false;

    if (!isEnabled) {
      logger.debug("Antigravity auto-rotation is disabled in config");
      return;
    }

    const intervalMs = Math.max(intervalMin, 1) * 60 * 1000;
    logger.info(`Starting Antigravity auto-rotator (interval: ${intervalMin}m)...`);

    this.timer = setInterval(async () => {
      await this.triggerRotation("timer");
    }, intervalMs);
  }

  stop(): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  async triggerRotation(reason: string = "manual"): Promise<void> {
    if (this.isRotating) return;
    this.isRotating = true;
    try {
      logger.info(`Triggering Antigravity account rotation (reason: ${reason})...`);
      const nextAcc = await tokenStore.rotateAntigravityAccount({ skipProcessCheck: true });
      if (nextAcc) {
        logger.info(`Auto-rotated Antigravity IDE account to: ${nextAcc.email}`);
      } else {
        logger.warn("Auto-rotation skipped: no eligible account available");
      }
    } catch (err) {
      logger.error("Auto-rotation failed:", err);
    } finally {
      this.isRotating = false;
    }
  }
}

export const autoRotator = new AutoRotator();
