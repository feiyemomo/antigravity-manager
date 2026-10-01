import { cac } from "cac";
import { startCommand } from "./commands/start.js";
import { loginCommand } from "./commands/login.js";
import { accountsCommand } from "./commands/accounts.js";
import { configCommand } from "./commands/config.js";
import { modelsCommand } from "./commands/models.js";
import { codeCommand } from "./commands/code.js";
import { doctorCommand } from "./commands/doctor.js";
import { profileCommand } from "./commands/profile.js";
import { setLogLevel } from "../shared/logger.js";
import { resolveProfile } from "../core/profileSwitcher.js";

const cli = cac("agy-tools");

// Global options
cli.option("--debug", "Enable debug logging");

// ============================================
// Doctor Command
// ============================================
cli
  .command("doctor", "Run diagnostic self-check on credentials, symlinks, and profiles")
  .option("--fix", "Automatically fix detected issues")
  .option("--json", "Output diagnostics as JSON")
  .action(doctorCommand);

// ============================================
// Multi-Account Profile Commands
// ============================================
cli
  .command("profiles", "List all profiles")
  .action(profileCommand.list);

cli
  .command("profile [action] [name]", "Manage profiles (list, switch, save, current, remove, link, doctor)")
  .option("-f, --force", "Force in-place hot-swap even if Antigravity is running")
  .option("--live", "Live hot-swap without closing Antigravity")
  .option("--restart-lsp", "Restart background language_server process immediately")
  .option("--fix", "Automatically fix detected issues (for doctor)")
  .option("--json", "Output diagnostics as JSON")
  .action(async (
    action?: string,
    name?: string,
    options?: { fix?: boolean; json?: boolean; live?: boolean; force?: boolean; restartLsp?: boolean }
  ) => {
    if (!action || action === "list" || action === "ls") {
      await profileCommand.list();
    } else if (action === "switch") {
      if (!name) {
        console.error("Usage: agy-tools profile switch <name> [--live]");
        process.exit(1);
      }
      await profileCommand.switch(name, options);
    } else if (action === "save" || action === "create" || action === "add") {
      if (!name) {
        console.error("Usage: agy-tools profile save <name>");
        process.exit(1);
      }
      await profileCommand.save(name);
    } else if (action === "current") {
      await profileCommand.current();
    } else if (action === "remove" || action === "rm" || action === "delete") {
      if (!name) {
        console.error("Usage: agy-tools profile remove <name>");
        process.exit(1);
      }
      await profileCommand.remove(name);
    } else if (action === "link") {
      await profileCommand.link();
    } else if (action === "doctor") {
      await doctorCommand(options);
    } else {
      console.error(`Unknown profile action: ${action}`);
      console.error("Available actions: list, switch, save, current, remove, link, doctor");
      process.exit(1);
    }
  });

// ============================================
// Core Commands & Shortcuts
// ============================================
async function unifiedSwitch(
  target: string,
  options?: { live?: boolean; force?: boolean; restartLsp?: boolean }
): Promise<void> {
  const resolved = resolveProfile(target);
  if (resolved) {
    await profileCommand.switch(resolved.name, options);
  } else {
    await accountsCommand.switch(target, options);
  }
}

cli
  .command("switch <id>", "Shortcut to atomically switch Antigravity account (profile name, account ID or email)")
  .option("-f, --force", "Force in-place hot-swap even if Antigravity is running")
  .option("--live", "Live hot-swap without closing Antigravity")
  .option("--restart-lsp", "Restart background language_server process immediately")
  .action(unifiedSwitch);

cli
  .command("rotate", "Shortcut to rotate Antigravity IDE account")
  .action(accountsCommand.rotate);

cli
  .command("start", "Start the proxy server")
  .option("-p, --port <port>", "Server port", { default: 38080 })
  .option("-H, --host <host>", "Server host", { default: "127.0.0.1" })
  .option("-k, --api-key <key>", "API key for authentication")
  .option("--open", "Open dashboard in browser automatically", { default: true })
  .action(startCommand);

cli
  .command("login", "Login with Google account (OAuth2)")
  .action(loginCommand);

cli
  .command("accounts", "List all accounts")
  .alias("ls")
  .action(accountsCommand.list);

cli
  .command("accounts add", "Add a new account (alias for login)")
  .action(loginCommand);

cli
  .command("accounts remove <id>", "Remove an account")
  .action(accountsCommand.remove);

cli
  .command("accounts refresh [id]", "Refresh account tokens")
  .action(accountsCommand.refresh);

cli
  .command("accounts switch <id>", "Switch Antigravity IDE account to specified ID, Email, or Profile")
  .option("-f, --force", "Force in-place hot-swap even if Antigravity is running")
  .option("--live", "Live hot-swap without closing Antigravity")
  .option("--restart-lsp", "Restart background language_server process immediately")
  .action(unifiedSwitch);

cli
  .command("accounts rotate", "Rotate Antigravity IDE account to the next available account")
  .action(accountsCommand.rotate);

cli
  .command("config [action] [key] [value]", "Manage configuration")
  .action((action?: string, key?: string, value?: string) => {
    if (!action || action === "show") {
      configCommand.show();
    } else if (action === "set") {
      if (!key || !value) {
        console.error("Usage: agy-tools config set <key> <value>");
        process.exit(1);
      }
      configCommand.set(key, value);
    } else if (action === "reset") {
      configCommand.reset();
    } else {
      console.error(`Unknown config action: ${action}`);
      process.exit(1);
    }
  });

cli
  .command("models", "List all available models")
  .action(modelsCommand);

cli
  .command("code <agent> [...args]", "Launch a coding agent with agy-tools proxy")
  .option("-p, --port <port>", "Server port (random if not specified)")
  .option("-H, --host <host>", "Server host", { default: "127.0.0.1" })
  .option("-k, --api-key <key>", "API key for authentication (random if not specified)")
  .action(codeCommand);

// Help and version
cli.help();
cli.version("0.2.0");

// Parse and run
export function run(): void {
  // If user double-clicks or runs without any subcommands, default to "start"
  const args = process.argv.slice(2);
  if (args.length === 0) {
    process.argv.push("start");
  }

  const parsed = cli.parse();

  // Handle global options
  if (parsed.options.debug) {
    setLogLevel("debug");
  }
}
