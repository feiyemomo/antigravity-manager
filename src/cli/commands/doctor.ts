import chalk from "chalk";
import ora from "ora";
import { runDiagnostics, fixDoctorIssues } from "../../core/doctorService.js";
import { logger } from "../../shared/index.js";
import type { DoctorCheckItem } from "../../core/types.js";

function renderCheckItem(item: DoctorCheckItem): void {
  let badge: string;
  let titleColor: (s: string) => string;

  switch (item.status) {
    case "ok":
      badge = chalk.bgGreen.black("  OK  ");
      titleColor = chalk.green.bold;
      break;
    case "warn":
      badge = chalk.bgYellow.black(" WARN ");
      titleColor = chalk.yellow.bold;
      break;
    case "error":
      badge = chalk.bgRed.white(" FAIL ");
      titleColor = chalk.red.bold;
      break;
  }

  logger.print(` ${badge} ${titleColor(item.name)}: ${item.message}`);

  if (item.details && item.details.length > 0) {
    for (const d of item.details) {
      logger.print(chalk.gray(`      ${d}`));
    }
  }

  if (item.remediation && item.status !== "ok") {
    logger.print(chalk.cyan(`      Fix: ${item.remediation}`));
  }

  logger.print("");
}

export async function doctorCommand(options?: { fix?: boolean; json?: boolean }): Promise<void> {
  const isJson = !!options?.json;
  const spinner = isJson ? null : ora("Running Antigravity multi-account diagnostics...").start();

  try {
    const report = await runDiagnostics();
    if (spinner) spinner.stop();

    if (isJson) {
      console.log(JSON.stringify(report, null, 2));
      return;
    }

    logger.print(chalk.bold(`\n=== Antigravity Multi-Account Diagnostic Report ===\n`));

    // 1. Process Safety
    renderCheckItem(report.checks.processes);

    // 2. Credential Storage
    renderCheckItem(report.checks.credentials);

    // 3. Profiles Storage
    renderCheckItem(report.checks.profiles);

    // 4. Shared Symlinks
    renderCheckItem(report.checks.symlinks);

    // Overall summary
    logger.print(chalk.bold("--- Summary ---"));
    let statusText: string;
    if (report.overallStatus === "healthy") {
      statusText = chalk.green.bold("HEALTHY - Ready for seamless account switching.");
    } else if (report.overallStatus === "degraded") {
      statusText = chalk.yellow.bold("DEGRADED - Functional, but some issues or running processes were detected.");
    } else {
      statusText = chalk.red.bold("CRITICAL - One or more components require attention.");
    }

    logger.print(`Status: ${statusText}\n`);

    // Handle --fix
    if (options?.fix) {
      const fixSpinner = ora("Attempting automated remediation (--fix)...").start();
      const fixResult = await fixDoctorIssues();

      if (fixResult.fixed.length > 0 || fixResult.failed.length > 0) {
        fixSpinner.stop();
        logger.print(chalk.bold("\n--- Automated Fix Results ---"));
        for (const f of fixResult.fixed) {
          logger.print(chalk.green(`  ✓ ${f}`));
        }
        for (const f of fixResult.failed) {
          logger.print(chalk.red(`  ✗ ${f}`));
        }
        logger.print("");
      } else {
        fixSpinner.succeed("No automated fixes needed or applicable.");
      }
    } else if (report.overallStatus !== "healthy") {
      logger.print(chalk.gray("Tip: Run 'agy-tools doctor --fix' to automatically configure default profiles and symlinks.\n"));
    }
  } catch (error) {
    if (spinner) {
      spinner.fail("Diagnostics failed unexpectedly");
    }
    logger.error("Error:", error);
    process.exit(1);
  }
}
