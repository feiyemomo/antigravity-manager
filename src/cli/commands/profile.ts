import chalk from "chalk";
import ora from "ora";
import { logger } from "../../shared/index.js";
import {
  captureCurrentAsProfile,
  deleteProfile,
  getActiveProfileName,
  getProfile,
  listProfiles,
} from "../../core/profileManager.js";
import { switchProfile } from "../../core/profileSwitcher.js";
import { ensureAllSharedLinks } from "../../core/sharedLinksManager.js";
import { ProcessSafetyError, restartLanguageServer } from "../../core/processGuard.js";

export const profileCommand = {
  async list(): Promise<void> {
    const profiles = listProfiles();
    const activeName = getActiveProfileName();

    if (profiles.length === 0) {
      logger.info("No profiles found. Use 'agy-tools profile save <name>' to save current account as a profile.");
      return;
    }

    logger.print(chalk.bold(`\nProfiles (${profiles.length}):\n`));

    for (const p of profiles) {
      const isActive = p.name === activeName;
      const activeTag = isActive ? chalk.bgGreen.black(" ACTIVE ") + " " : "        ";
      const emailDisplay = p.email ? chalk.cyan(p.email) : chalk.gray("no-email");
      const tokenStatus = p.isTokenExpired
        ? chalk.red("token: expired")
        : chalk.green("token: valid");
      const healthTag = p.hasInstallationId && p.hasToken && p.hasSettings
        ? chalk.green("complete")
        : chalk.yellow("incomplete");

      logger.print(
        `  ${activeTag} ${chalk.bold.white(p.name.padEnd(16))} ${emailDisplay.padEnd(30)} [${healthTag}] [${tokenStatus}]`
      );

      if (p.lastUsedAt) {
        logger.print(chalk.gray(`             Last used: ${new Date(p.lastUsedAt).toLocaleString()}`));
      }
    }

    logger.print("");
  },

  async current(): Promise<void> {
    const activeName = getActiveProfileName();
    if (!activeName) {
      logger.warn("No profile is currently marked as active.");
      return;
    }

    const payload = getProfile(activeName);
    if (!payload) {
      logger.error(`Active profile '${activeName}' metadata could not be found.`);
      return;
    }

    logger.print(chalk.bold(`\nActive Profile: ${chalk.green(activeName)}`));
    if (payload.metadata?.email) {
      logger.print(`Email: ${chalk.cyan(payload.metadata.email)}`);
    }
    logger.print(`Installation ID: ${chalk.gray(payload.installationId)}`);
    if (payload.metadata?.lastUsedAt) {
      logger.print(`Last Used: ${new Date(payload.metadata.lastUsedAt).toLocaleString()}`);
    }
    logger.print("");
  },

  async switch(
    nameOrQuery: string,
    options?: { live?: boolean; force?: boolean; restartLsp?: boolean }
  ): Promise<void> {
    if (!nameOrQuery) {
      logger.error("Usage: agy-tools profile switch <name> [--live]");
      process.exit(1);
    }

    const isLive = !!(options?.live || options?.force);
    const spinner = ora(
      isLive
        ? `Hot-swapping Antigravity profile to '${nameOrQuery}' (Live mode)...`
        : `Switching Antigravity profile to '${nameOrQuery}'...`
    ).start();

    try {
      const res = await switchProfile(nameOrQuery, { skipProcessCheck: isLive });

      let lspMessage = "";
      if (options?.restartLsp) {
        const restarted = restartLanguageServer();
        lspMessage = restarted ? " (language_server reloaded)" : "";
      }

      spinner.succeed(
        `Switched Antigravity account to profile: ${chalk.green.bold(res.profileName)}` +
        (res.email ? ` (${res.email})` : "") +
        (isLive ? chalk.yellow(" [LIVE HOT-SWAP]") : "") +
        lspMessage
      );

      if (isLive) {
        logger.print(
          chalk.gray(
            "Tip: Live hot-swap has updated Credential Manager & isolated token files.\n" +
            "     Antigravity background services will apply the new credentials on your next prompt."
          )
        );
      }
    } catch (error) {
      spinner.fail("Failed to switch profile");

      if (error instanceof ProcessSafetyError) {
        logger.print(chalk.red.bold("\n" + error.message + "\n"));
      } else {
        logger.error("Error:", error);
      }
      process.exit(1);
    }
  },

  async save(name: string): Promise<void> {
    if (!name) {
      logger.error("Usage: agy-tools profile save <name>");
      process.exit(1);
    }

    const spinner = ora(`Capturing current Antigravity environment as profile '${name}'...`).start();

    try {
      const profile = await captureCurrentAsProfile(name);
      spinner.succeed(
        `Profile '${chalk.green.bold(name)}' saved successfully!` +
        (profile.email ? ` (Account: ${profile.email})` : "")
      );
    } catch (error) {
      spinner.fail("Failed to save profile");
      logger.error("Error:", error);
      process.exit(1);
    }
  },

  async remove(name: string): Promise<void> {
    if (!name) {
      logger.error("Usage: agy-tools profile remove <name>");
      process.exit(1);
    }

    const success = deleteProfile(name);
    if (success) {
      logger.success(`Removed profile: ${name}`);
    } else {
      logger.error(`Profile not found: ${name}`);
      process.exit(1);
    }
  },

  async link(): Promise<void> {
    const spinner = ora("Setting up global shared symlinks (conversations, skills)...").start();

    try {
      const res = ensureAllSharedLinks();
      if (res.errors.length > 0) {
        spinner.warn(`Linked ${res.healthy}/${res.total} directories with some warnings.`);
        for (const e of res.errors) {
          logger.warn(`  - ${e}`);
        }
      } else {
        spinner.succeed(
          `All ${res.total} shared directories are linked via symbolic links.` +
          (res.migrated > 0 ? ` Migrated ${res.migrated} directory.` : "")
        );
      }
    } catch (error) {
      spinner.fail("Failed to link shared directories");
      logger.error("Error:", error);
      process.exit(1);
    }
  },
};
