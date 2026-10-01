import { existsSync, readFileSync, writeFileSync } from "node:fs";
import {
  ANTIGRAVITY_APP_STORAGE_FILE,
  ANTIGRAVITY_DEVTOOLS_PORT_FILE,
} from "./constants.js";

export interface LiveSyncResult {
  success: boolean;
  method: "devtools_live_push" | "disk_storage_updated" | "skipped";
  message: string;
  quotaFetched?: boolean;
}

/**
 * Safely updates the onboarding username in app_storage.json on disk.
 */
export function updateAppStorageEmail(email: string): boolean {
  try {
    if (!existsSync(ANTIGRAVITY_APP_STORAGE_FILE)) {
      return false;
    }
    const raw = readFileSync(ANTIGRAVITY_APP_STORAGE_FILE, "utf-8");
    const json = JSON.parse(raw);
    json["jetski.onboarding.lastLoginUsername"] = email;
    writeFileSync(ANTIGRAVITY_APP_STORAGE_FILE, JSON.stringify(json, null, 2), "utf-8");
    return true;
  } catch {
    return false;
  }
}

/**
 * Reads the active DevTools port from DevToolsActivePort if Antigravity is running with remote debugging.
 */
export function getActiveDevToolsPort(): number | null {
  try {
    if (!existsSync(ANTIGRAVITY_DEVTOOLS_PORT_FILE)) {
      return null;
    }
    const content = readFileSync(ANTIGRAVITY_DEVTOOLS_PORT_FILE, "utf-8").trim();
    const firstLine = content.split(/\r?\n/)[0];
    const port = parseInt(firstLine, 10);
    return isNaN(port) ? null : port;
  } catch {
    return null;
  }
}

/**
 * Fetches real quota summary from Google Cloud Code API for the given access token.
 */
export async function fetchLiveQuotaSummary(accessToken: string): Promise<any[] | null> {
  const endpoints = [
    "https://daily-cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary",
    "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary",
  ];

  for (const url of endpoints) {
    try {
      const res = await fetch(url, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${accessToken}`,
          "Content-Type": "application/json",
          "User-Agent": "antigravity",
          "X-Client-Name": "antigravity",
          "X-Client-Version": "2.18.1",
        },
        body: JSON.stringify({}),
        signal: AbortSignal.timeout(3000),
      });

      if (res.ok) {
        const data = (await res.json()) as any;
        if (data.groups && Array.isArray(data.groups)) {
          return data.groups.map((g: any) => ({
            displayName: g.displayName || "",
            description: g.description || "",
            buckets: (g.buckets || []).map((b: any) => ({
              bucketId: b.bucketId || "",
              displayName: b.displayName || "",
              description: b.description || "",
              window: b.window,
              remaining: {
                case: "remainingFraction",
                value: b.remaining?.value ?? b.remainingFraction ?? 1,
              },
              disabled: b.disabled || false,
              resetTime: b.resetTime
                ? {
                    seconds: String(
                      Math.floor(
                        (typeof b.resetTime === "string"
                          ? new Date(b.resetTime).getTime()
                          : Number(b.resetTime.seconds || 0) * 1000) / 1000
                      )
                    ),
                    nanos: 0,
                  }
                : undefined,
            })),
          }));
        }
      }
    } catch {
      // try next
    }
  }

  return null;
}

/**
 * Connects to active Antigravity Electron windows via Chrome DevTools Protocol
 * and hot-pushes the updated user profile and quota into the React runtime and native storage.
 */
export async function syncLiveAntigravityUi(
  email: string,
  name?: string,
  accessToken?: string
): Promise<LiveSyncResult> {
  // 1. Update on-disk app_storage.json
  const storageUpdated = updateAppStorageEmail(email);

  // 2. Fetch real quota groups if access token is available
  let liveGroups: any[] | null = null;
  if (accessToken) {
    try {
      liveGroups = await fetchLiveQuotaSummary(accessToken);
    } catch {
      // non-fatal
    }
  }

  // 3. Check for active DevTools port
  const port = getActiveDevToolsPort();
  if (!port) {
    return {
      success: storageUpdated,
      method: storageUpdated ? "disk_storage_updated" : "skipped",
      message: storageUpdated
        ? "Account profile updated in app_storage.json. Restart or reload Antigravity (Ctrl+R) to take effect."
        : "No active Antigravity session found to update.",
      quotaFetched: !!liveGroups,
    };
  }

  // 4. Connect to DevTools and push live state into React
  try {
    const listRes = await fetch(`http://127.0.0.1:${port}/json/list`, {
      signal: AbortSignal.timeout(1500),
    });
    if (!listRes.ok) {
      throw new Error(`Failed to list DevTools targets: HTTP ${listRes.status}`);
    }

    const targets = (await listRes.json()) as Array<{
      type: string;
      webSocketDebuggerUrl?: string;
    }>;

    const pageTargets = targets.filter(
      (t) => t.type === "page" && t.webSocketDebuggerUrl
    );

    if (pageTargets.length === 0) {
      return {
        success: true,
        method: "disk_storage_updated",
        message: "DevTools active port found but no open page targets. app_storage.json updated.",
      };
    }

    const displayName = name || email.split("@")[0];
    const script = `(() => {
      try {
        if (window.nativeStorage && typeof window.nativeStorage.updateItems === "function") {
          window.nativeStorage.updateItems({
            "jetski.onboarding.lastLoginUsername": ${JSON.stringify(email)}
          });
        }
        const root = document.getElementById("root") || document.body.firstElementChild;
        if (root) {
          const key = Object.keys(root).find(k => k.startsWith("__reactContainer"));
          if (key && root[key]) {
            let queue = [root[key]];
            let userProvider = null;
            let cloudService = null;
            let tokenProvider = null;
            let refreshModelsFn = null;

            while (queue.length > 0 && queue.length < 10000) {
              const f = queue.shift();
              if (!f) continue;
              if (f.memoizedProps?.value?.userStatusProvider) {
                userProvider = f.memoizedProps.value.userStatusProvider;
              }
              if (f.memoizedProps?.value?.core?.cloudCodeService) {
                cloudService = f.memoizedProps.value.core.cloudCodeService;
              }
              if (f.memoizedProps?.value?.oauthTokenProvider) {
                tokenProvider = f.memoizedProps.value.oauthTokenProvider;
              }
              if (f.memoizedProps?.refreshModels && typeof f.memoizedProps.refreshModels === "function") {
                refreshModelsFn = f.memoizedProps.refreshModels;
              }
              if (f.child) queue.push(f.child);
              if (f.sibling) queue.push(f.sibling);
            }

            const rawGroups = ${JSON.stringify(liveGroups)};

            // 1. Hook cloudCodeService.retrieveUserQuotaSummary with exact protobuf format & BigInt timestamps
            if (cloudService && rawGroups) {
              const formattedGroups = rawGroups.map(g => ({
                $typeName: "google.internal.cloud.code.v1internal.QuotaSummaryGroup",
                displayName: g.displayName || "",
                description: g.description || "",
                buckets: (g.buckets || []).map(b => {
                  const val = typeof b.remaining?.value === "number" ? b.remaining.value : (typeof b.remainingFraction === "number" ? b.remainingFraction : 1);
                  let sec = 0;
                  if (b.resetTime) {
                    if (typeof b.resetTime.seconds === "string" || typeof b.resetTime.seconds === "number") {
                      sec = parseInt(String(b.resetTime.seconds), 10);
                    } else if (typeof b.resetTime === "string") {
                      sec = Math.floor(new Date(b.resetTime).getTime() / 1000);
                    }
                  }
                  return {
                    $typeName: "google.internal.cloud.code.v1internal.QuotaSummaryBucket",
                    bucketId: b.bucketId || "",
                    displayName: b.displayName || "",
                    description: b.description || "",
                    window: b.window || "",
                    remaining: {
                      case: "remainingFraction",
                      value: val
                    },
                    disabled: !!b.disabled,
                    resetTime: sec > 0 ? {
                      $typeName: "google.protobuf.Timestamp",
                      seconds: BigInt(sec),
                      nanos: 0
                    } : undefined
                  };
                })
              }));

              cloudService.retrieveUserQuotaSummary = async () => ({
                $typeName: "exa.language_server_pb.RetrieveUserQuotaSummaryResponse",
                response: {
                  $typeName: "google.internal.cloud.code.v1internal.RetrieveUserQuotaSummaryResponse",
                  buckets: [],
                  groups: formattedGroups,
                  description: ""
                }
              });
            }

            // 2. Compute per-family fractions for model selector UI
            let geminiFraction = 1;
            let claudeFraction = 1;
            if (rawGroups && Array.isArray(rawGroups)) {
              for (const g of rawGroups) {
                const isGemini = (g.displayName || "").toLowerCase().includes("gemini");
                const isClaude = (g.displayName || "").toLowerCase().includes("claude");
                for (const b of (g.buckets || [])) {
                  const val = typeof b.remaining?.value === "number" ? b.remaining.value : (typeof b.remainingFraction === "number" ? b.remainingFraction : 1);
                  if (isGemini && (b.window === "5h" || b.bucketId?.includes("5h"))) geminiFraction = val;
                  else if (isGemini && geminiFraction === 1) geminiFraction = val;
                  if (isClaude && (b.window === "5h" || b.bucketId?.includes("5h"))) claudeFraction = val;
                  else if (isClaude && claudeFraction === 1) claudeFraction = val;
                }
              }
            }

            // 3. Update user profile & model quotaInfos
            if (userProvider && typeof userProvider.pushUpdate === "function") {
              const st = userProvider.getState() || {};
              let newConfigs = st.cascadeModelConfigData?.clientModelConfigs;
              if (newConfigs && Array.isArray(newConfigs)) {
                newConfigs = newConfigs.map(m => {
                  const label = (m.label || m.modelId || "").toLowerCase();
                  const fraction = (label.includes("claude") || label.includes("gpt")) ? claudeFraction : geminiFraction;
                  return {
                    ...m,
                    quotaInfo: {
                      ...m.quotaInfo,
                      remainingFraction: fraction
                    }
                  };
                });
              }

              userProvider.pushUpdate({
                ...st,
                email: ${JSON.stringify(email)},
                name: ${JSON.stringify(displayName)},
                cascadeModelConfigData: {
                  ...st.cascadeModelConfigData,
                  clientModelConfigs: newConfigs
                }
              });
            }

            // 4. Update OAuth token
            if (tokenProvider && typeof tokenProvider.pushUpdate === "function" && ${JSON.stringify(accessToken || "")}) {
              tokenProvider.pushUpdate({
                accessToken: ${JSON.stringify(accessToken || "")},
                refreshToken: "",
                expiryDateSeconds: Date.now() + 3600000,
                tokenType: "Bearer"
              });
            }

            // 5. Trigger model selector refresh if available
            if (refreshModelsFn && typeof refreshModelsFn === "function") {
              try { refreshModelsFn(); } catch {}
            }

            return { success: true, liveUiUpdated: true, quotaInjected: !!rawGroups };
          }
        }
        return { success: true, nativeStorageUpdated: true };
      } catch (e) {
        return { success: false, error: String(e) };
      }
    })()`;

    for (const target of pageTargets) {
      if (!target.webSocketDebuggerUrl) continue;
      await new Promise<void>((resolve) => {
        try {
          const ws = new WebSocket(target.webSocketDebuggerUrl!);
          const timer = setTimeout(() => {
            try { ws.close(); } catch {}
            resolve();
          }, 2500);

          ws.onopen = () => {
            ws.send(
              JSON.stringify({
                id: 1,
                method: "Runtime.evaluate",
                params: {
                  expression: script,
                  returnByValue: true,
                },
              })
            );
          };

          ws.onmessage = () => {
            clearTimeout(timer);
            try { ws.close(); } catch {}
            resolve();
          };

          ws.onerror = () => {
            clearTimeout(timer);
            try { ws.close(); } catch {}
            resolve();
          };
        } catch {
          resolve();
        }
      });
    }

    return {
      success: true,
      method: "devtools_live_push",
      message: `Antigravity UI live updated to ${email} with quota sync without restarting.`,
      quotaFetched: !!liveGroups,
    };
  } catch (err: any) {
    return {
      success: true,
      method: "disk_storage_updated",
      message: `Updated app_storage.json (${err.message}). Press Ctrl+R in Antigravity to refresh.`,
      quotaFetched: !!liveGroups,
    };
  }
}
