import { Hono } from "hono";
import { tokenStore } from "../services/tokenStore.js";
import { loadConfig, updateConfig } from "../../shared/config.js";
import { startOAuthFlow } from "../services/auth.js";
import { autoRotator } from "../services/autoRotator.js";
import { logger } from "../../shared/logger.js";
import type { Account } from "../../shared/types.js";

let loginState: {
  status: "idle" | "waiting" | "success" | "error";
  account?: Account;
  error?: string;
} = { status: "idle" };

export function setupApiRoutes(app: Hono): void {
  const api = new Hono();

  // 1. 获取所有账号及当前激活账号
  api.get("/accounts", async (c) => {
    await tokenStore.load();
    const accounts = tokenStore.getAccounts();
    // Auto-refresh quota for any accounts missing quota or groups
    for (const acc of accounts) {
      if (!acc.quota || !acc.quota.groups || acc.quota.groups.length === 0) {
        tokenStore.refreshQuota(acc.id).catch(() => {});
      }
    }
    return c.json({
      accounts: tokenStore.getAccounts(),
      currentAccountId: tokenStore.getCurrentAccountId(),
    });
  });

  // 2. 一键切换 Antigravity IDE 账号 (Web 控制台支持在线热切换)
  api.post("/antigravity/switch", async (c) => {
    try {
      const body = await c.req.json().catch(() => ({}));
      const id = body.id;
      if (!id) {
        return c.json({ success: false, error: "Missing account id" }, 400);
      }
      const account = await tokenStore.switchAntigravityAccount(id, { skipProcessCheck: true });
      return c.json({ success: true, account });
    } catch (err) {
      return c.json({ success: false, error: err instanceof Error ? err.message : String(err) }, 500);
    }
  });

  // 3. 立即轮换 Antigravity IDE 到下一个可用账号 (Web 控制台支持在线热轮换)
  api.post("/antigravity/rotate", async (c) => {
    try {
      const account = await tokenStore.rotateAntigravityAccount({ skipProcessCheck: true });
      if (!account) {
        return c.json({ success: false, error: "No eligible accounts found" }, 400);
      }
      return c.json({ success: true, account });
    } catch (err) {
      return c.json({ success: false, error: err instanceof Error ? err.message : String(err) }, 500);
    }
  });

  // 4. 获取与更新系统配置
  api.get("/config", (c) => {
    return c.json(loadConfig());
  });

  api.post("/config", async (c) => {
    try {
      const body = await c.req.json().catch(() => ({}));
      const updated = updateConfig(body);
      // 如果更新了定时轮换配置，重启轮换器
      autoRotator.start();
      return c.json({ success: true, config: updated });
    } catch (err) {
      return c.json({ success: false, error: err instanceof Error ? err.message : String(err) }, 500);
    }
  });

  // 5. 账号单项操作：刷新、删除
  api.post("/accounts/:id/refresh", async (c) => {
    const id = c.req.param("id");
    try {
      await tokenStore.refreshAccount(id);
      await tokenStore.refreshQuota(id);
      return c.json({ success: true });
    } catch (err) {
      return c.json({ success: false, error: err instanceof Error ? err.message : String(err) }, 500);
    }
  });

  api.post("/accounts/refresh-all", async (c) => {
    try {
      const accounts = tokenStore.getAccounts();
      for (const acc of accounts) {
        try {
          await tokenStore.refreshAccount(acc.id);
          await tokenStore.refreshQuota(acc.id);
        } catch {}
      }
      return c.json({ success: true });
    } catch (err) {
      return c.json({ success: false, error: err instanceof Error ? err.message : String(err) }, 500);
    }
  });

  api.delete("/accounts/:id", async (c) => {
    const id = c.req.param("id");
    const ok = await tokenStore.removeAccount(id);
    return c.json({ success: ok });
  });

  // 6. Web 端 Google 登录 OAuth 流程
  api.post("/auth/start", async (c) => {
    try {
      const { authUrl, waitForCallback } = await startOAuthFlow();
      loginState = { status: "waiting" };

      waitForCallback()
        .then((account) => {
          loginState = { status: "success", account };
          logger.info(`Web login success for: ${account.email}`);
        })
        .catch((err) => {
          loginState = { status: "error", error: err instanceof Error ? err.message : String(err) };
          logger.warn(`Web login failed: ${err.message}`);
        });

      return c.json({ success: true, url: authUrl });
    } catch (err) {
      return c.json({ success: false, error: err instanceof Error ? err.message : String(err) }, 500);
    }
  });

  api.get("/auth/status", (c) => {
    return c.json(loginState);
  });

  app.route("/api", api);
}
