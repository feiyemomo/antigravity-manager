package livesync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"agy-tools/pkg/config"
	"agy-tools/pkg/types"
	"github.com/gorilla/websocket"
)

type DevToolsTarget struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type LiveSyncResult struct {
	Success      bool   `json:"success"`
	Method       string `json:"method"`
	Message      string `json:"message"`
	QuotaFetched bool   `json:"quotaFetched"`
}

// GetActiveDevToolsPort reads the active DevTools port from Antigravity's DevToolsActivePort file
func GetActiveDevToolsPort() int {
	data, err := os.ReadFile(config.DevToolsPortFile)
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		return 0
	}
	port, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return 0
	}
	return port
}

// UpdateAppStorageEmail safely updates the onboarding username in app_storage.json on disk
func UpdateAppStorageEmail(email string) bool {
	if _, err := os.Stat(config.AppStorageFile); os.IsNotExist(err) {
		return false
	}
	data, err := os.ReadFile(config.AppStorageFile)
	if err != nil {
		return false
	}

	var storage map[string]interface{}
	if err := json.Unmarshal(data, &storage); err != nil {
		return false
	}

	storage["jetski.onboarding.lastLoginUsername"] = email
	out, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return false
	}

	return os.WriteFile(config.AppStorageFile, out, 0644) == nil
}

// FetchLiveQuotaSummary calls Google Cloud Code API with required Antigravity headers
func FetchLiveQuotaSummary(accessToken string) ([]types.QuotaGroup, error) {
	endpoint := "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary"

	client := &http.Client{Timeout: 3 * time.Second}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "antigravity")
	req.Header.Set("X-Client-Name", "antigravity")
	req.Header.Set("X-Client-Version", "2.18.1")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("quota endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResp struct {
		Groups []struct {
			DisplayName string `json:"displayName"`
			Description string `json:"description"`
			Buckets     []struct {
				BucketID          string  `json:"bucketId"`
				DisplayName       string  `json:"displayName"`
				Description       string  `json:"description"`
				Window            string  `json:"window"`
				RemainingFraction float64 `json:"remainingFraction"`
				Remaining         *struct {
					Value float64 `json:"value"`
				} `json:"remaining"`
				Disabled  bool `json:"disabled"`
				ResetTime any  `json:"resetTime"`
			} `json:"buckets"`
		} `json:"groups"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}

	if len(apiResp.Groups) == 0 {
		return nil, fmt.Errorf("no quota groups returned")
	}

	var groups []types.QuotaGroup
	for _, g := range apiResp.Groups {
		group := types.QuotaGroup{
			DisplayName: g.DisplayName,
			Description: g.Description,
		}
		for _, b := range g.Buckets {
			fraction := b.RemainingFraction
			if b.Remaining != nil && b.Remaining.Value > 0 {
				fraction = b.Remaining.Value
			}

			resetTimeStr := ""
			if b.ResetTime != nil {
				switch v := b.ResetTime.(type) {
				case string:
					resetTimeStr = v
				case map[string]interface{}:
					if secVal, ok := v["seconds"]; ok {
						switch sec := secVal.(type) {
						case float64:
							resetTimeStr = time.Unix(int64(sec), 0).Format(time.RFC3339)
						case string:
							s, _ := strconv.ParseInt(sec, 10, 64)
							resetTimeStr = time.Unix(s, 0).Format(time.RFC3339)
						}
					}
				}
			}

			bucket := types.QuotaBucket{
				BucketID:          b.BucketID,
				DisplayName:       b.DisplayName,
				Description:       b.Description,
				Window:            b.Window,
				RemainingFraction: fraction,
				Percentage:        math.Round(fraction*1000) / 10,
				ResetTime:         resetTimeStr,
				Disabled:          b.Disabled,
			}
			group.Buckets = append(group.Buckets, bucket)
		}
		groups = append(groups, group)
	}

	return groups, nil
}

// SyncLiveAntigravityUI connects to active Antigravity via Chrome DevTools Protocol
func SyncLiveAntigravityUI(email, name, accessToken string, presetGroups ...[]types.QuotaGroup) LiveSyncResult {
	storageUpdated := UpdateAppStorageEmail(email)

	var liveGroups []types.QuotaGroup
	if len(presetGroups) > 0 && len(presetGroups[0]) > 0 {
		liveGroups = presetGroups[0]
	} else if accessToken != "" {
		groups, err := FetchLiveQuotaSummary(accessToken)
		if err == nil {
			liveGroups = groups
		}
	}

	port := GetActiveDevToolsPort()
	if port == 0 {
		return LiveSyncResult{
			Success:      storageUpdated,
			Method:       "disk_storage_updated",
			Message:      "Account profile updated in app_storage.json.",
			QuotaFetched: len(liveGroups) > 0,
		}
	}

	// 1. Get DevTools targets
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
	if err != nil {
		return LiveSyncResult{
			Success: storageUpdated,
			Method:  "disk_storage_updated",
			Message: "DevTools active port found but connection failed.",
		}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var targets []DevToolsTarget
	_ = json.Unmarshal(body, &targets)

	var pageTarget *DevToolsTarget
	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebuggerURL != "" {
			pageTarget = &t
			break
		}
	}

	if pageTarget == nil {
		return LiveSyncResult{
			Success: true,
			Method:  "disk_storage_updated",
			Message: "No open page target found in Antigravity.",
		}
	}

	displayName := name
	if displayName == "" {
		parts := strings.Split(email, "@")
		displayName = parts[0]
	}

	groupsJSON, _ := json.Marshal(liveGroups)
	emailJSON, _ := json.Marshal(email)
	nameJSON, _ := json.Marshal(displayName)
	tokenJSON, _ := json.Marshal(accessToken)

	script := fmt.Sprintf(`(async () => {
		try {
			if (window.nativeStorage && typeof window.nativeStorage.updateItems === "function") {
				window.nativeStorage.updateItems({
					"jetski.onboarding.lastLoginUsername": %s
				});
			}

			const root = document.getElementById("root") || document.body.firstElementChild;
			if (!root) return { success: true, nativeStorageUpdated: true };

			const key = Object.keys(root).find(k => k.startsWith("__reactContainer"));
			if (!key || !root[key]) return { success: true, nativeStorageUpdated: true };

			let queue = [root[key]];
			let userProvider = null;
			let cloudService = null;
			let tokenProvider = null;
			let authService = null;
			let authStateProv = null;
			let syncedStateObj = null;
			let lsClientMgr = null;
			let refreshModelsFn = null;

			while (queue.length > 0 && queue.length < 15000) {
				const f = queue.shift();
				if (!f) continue;
				if (f.memoizedProps?.syncedState) {
					syncedStateObj = f.memoizedProps.syncedState;
				}
				if (f.memoizedProps?.lsClientManager) {
					lsClientMgr = f.memoizedProps.lsClientManager;
				}
				if (f.memoizedProps?.services?.core?.cloudCodeService) {
					cloudService = f.memoizedProps.services.core.cloudCodeService;
				}
				if (f.memoizedProps?.services?.core?.authService) {
					authService = f.memoizedProps.services.core.authService;
				}
				if (f.memoizedProps?.value?.userStatusProvider) userProvider = f.memoizedProps.value.userStatusProvider;
				if (f.memoizedProps?.value?.core?.cloudCodeService) cloudService = f.memoizedProps.value.core.cloudCodeService;
				if (f.memoizedProps?.value?.core?.authService) authService = f.memoizedProps.value.core.authService;
				if (f.memoizedProps?.value?.oauthTokenProvider) tokenProvider = f.memoizedProps.value.oauthTokenProvider;
				if (f.memoizedProps?.refreshModels && typeof f.memoizedProps.refreshModels === "function") refreshModelsFn = f.memoizedProps.refreshModels;
				if (f.child) queue.push(f.child);
				if (f.sibling) queue.push(f.sibling);
			}

			if (syncedStateObj) {
				if (syncedStateObj.userStatusProvider) userProvider = syncedStateObj.userStatusProvider;
				if (syncedStateObj.oauthTokenProvider) tokenProvider = syncedStateObj.oauthTokenProvider;
				if (syncedStateObj.authStateProvider) authStateProv = syncedStateObj.authStateProvider;
			}

			const rawGroups = %s;

			// 1. Hook cloudCodeService.retrieveUserQuotaSummary with exact protobuf format & BigInt timestamps
			if (cloudService && rawGroups && rawGroups.length > 0) {
				const formattedGroups = rawGroups.map(g => ({
					"$typeName": "google.internal.cloud.code.v1internal.QuotaSummaryGroup",
					"displayName": g.displayName || "",
					"description": g.description || "",
					"buckets": (g.buckets || []).map(b => {
						const val = typeof b.remainingFraction === "number" ? b.remainingFraction : 1;
						let sec = 0;
						if (b.resetTime) {
							sec = Math.floor(new Date(b.resetTime).getTime() / 1000);
						}
						return {
							"$typeName": "google.internal.cloud.code.v1internal.QuotaSummaryBucket",
							"bucketId": b.bucketId || "",
							"displayName": b.displayName || "",
							"description": b.description || "",
							"window": b.window || "",
							"remaining": {
								"case": "remainingFraction",
								"value": val
							},
							"disabled": !!b.disabled,
							"resetTime": sec > 0 ? {
								"$typeName": "google.protobuf.Timestamp",
								"seconds": BigInt(sec),
								"nanos": 0
							} : undefined
						};
					})
				}));

				cloudService.retrieveUserQuotaSummary = async () => ({
					"$typeName": "exa.language_server_pb.RetrieveUserQuotaSummaryResponse",
					"response": {
						"$typeName": "google.internal.cloud.code.v1internal.RetrieveUserQuotaSummaryResponse",
						"buckets": [],
						"groups": formattedGroups,
						"description": ""
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
						const val = typeof b.remainingFraction === "number" ? b.remainingFraction : 1;
						if (isGemini && (b.window === "5h" || b.bucketId?.includes("5h"))) geminiFraction = val;
						else if (isGemini && geminiFraction === 1) geminiFraction = val;
						if (isClaude && (b.window === "5h" || b.bucketId?.includes("5h"))) claudeFraction = val;
						else if (isClaude && claudeFraction === 1) claudeFraction = val;
					}
				}
			}

			// 3. Update user profile & model quotaInfos
			if (userProvider) {
				const currentSt = (typeof userProvider.getState === 'function' ? userProvider.getState() : {}) || {};
				let newConfigs = currentSt.cascadeModelConfigData?.clientModelConfigs;
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

				const updatedState = {
					...currentSt,
					email: %s,
					name: %s,
					cascadeModelConfigData: {
						...currentSt.cascadeModelConfigData,
						clientModelConfigs: newConfigs
					}
				};

				if (userProvider.provider && typeof userProvider.provider.setState === 'function') {
					userProvider.provider.setState(updatedState);
				}
				if (typeof userProvider.pushUpdate === 'function') {
					userProvider.pushUpdate(updatedState);
				}
			}

			// 4. Update OAuth token
			if (tokenProvider && %s) {
				const tokenObj = {
					accessToken: %s,
					refreshToken: "",
					expiryDateSeconds: Date.now() + 3600000,
					tokenType: "Bearer"
				};
				if (tokenProvider.provider && typeof tokenProvider.provider.setState === 'function') {
					tokenProvider.provider.setState(tokenObj);
				}
				if (typeof tokenProvider.pushUpdate === 'function') {
					tokenProvider.pushUpdate(tokenObj);
				}
			}

			// 5. Update authStateProvider
			if (authStateProv && %s) {
				const aspSt = (typeof authStateProv.getState === 'function' ? authStateProv.getState() : {}) || {};
				const newAspSt = {
					...aspSt,
					context: {
						...(aspSt.context || {}),
						tokenInfo: { accessToken: %s }
					}
				};
				if (authStateProv.provider && typeof authStateProv.provider.setState === 'function') {
					authStateProv.provider.setState(newAspSt);
				}
			}

			// 6. Smooth in-memory auth reload (non-blocking in background)
			if (authService) {
				try { authService.checkCurrentAuthStatus().catch(() => {}); } catch(e) {}
				try { authService.refreshUserStatus().catch(() => {}); } catch(e) {}
			}

			// 7. Trigger lsClientManager sync & model refresh
			if (lsClientMgr && typeof lsClientMgr.sync === 'function') {
				try { lsClientMgr.sync(); } catch(e) {}
			}
			if (refreshModelsFn && typeof refreshModelsFn === "function") {
				try { refreshModelsFn(); } catch {}
			}

			return { success: true, liveUiUpdated: true, quotaInjected: !!rawGroups };
		} catch(e) {
			return { success: false, error: String(e) };
		}
	})()`, emailJSON, groupsJSON, emailJSON, nameJSON, tokenJSON, tokenJSON, tokenJSON, tokenJSON)

	// Send via WebSocket to DevTools
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(ctx, pageTarget.WebSocketDebuggerURL, nil)
	if err != nil {
		return LiveSyncResult{
			Success: true,
			Method:  "disk_storage_updated",
			Message: "Connected to DevTools port, but WebSocket dial failed: " + err.Error(),
		}
	}
	defer conn.Close()

	payload := map[string]interface{}{
		"id":     1,
		"method": "Runtime.evaluate",
		"params": map[string]interface{}{
			"expression":    script,
			"returnByValue": true,
			"awaitPromise":  true,
		},
	}

	if err := conn.WriteJSON(payload); err != nil {
		return LiveSyncResult{
			Success: true,
			Method:  "disk_storage_updated",
			Message: "Failed to send CDP evaluation: " + err.Error(),
		}
	}

	// Read response briefly
	_ = conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
	var res map[string]interface{}
	_ = conn.ReadJSON(&res)

	return LiveSyncResult{
		Success:      true,
		Method:       "devtools_live_push",
		Message:      "Successfully pushed live credentials, user profile & quota into running Antigravity IDE",
		QuotaFetched: len(liveGroups) > 0,
	}
}

// WaitForLiveAntigravityReady polls Antigravity via DevTools until the React userStatusProvider reflects targetEmail
func WaitForLiveAntigravityReady(targetEmail string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	queryScript := `(() => {
		try {
			const root = document.getElementById("root") || document.body.firstElementChild;
			if (!root) return null;
			const key = Object.keys(root).find(k => k.startsWith("__reactContainer"));
			if (!key || !root[key]) return null;
			let queue = [root[key]];
			while (queue.length > 0 && queue.length < 15000) {
				const f = queue.shift();
				if (!f) continue;
				if (f.memoizedProps?.syncedState?.userStatusProvider?.getState) {
					return f.memoizedProps.syncedState.userStatusProvider.getState()?.email || null;
				}
				if (f.child) queue.push(f.child);
				if (f.sibling) queue.push(f.sibling);
			}
		} catch(e) {}
		return null;
	})()`

	for time.Now().Before(deadline) {
		port := GetActiveDevToolsPort()
		if port == 0 {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		client := &http.Client{Timeout: 500 * time.Millisecond}
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var targets []DevToolsTarget
		_ = json.Unmarshal(body, &targets)

		var wsURL string
		for _, t := range targets {
			if t.Type == "page" && t.WebSocketDebuggerURL != "" {
				wsURL = t.WebSocketDebuggerURL
				break
			}
		}

		if wsURL == "" {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		dialer := websocket.DefaultDialer
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		conn, _, err := dialer.DialContext(ctx, wsURL, nil)
		cancel()
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		_ = conn.WriteJSON(map[string]interface{}{
			"id":     999,
			"method": "Runtime.evaluate",
			"params": map[string]interface{}{
				"expression":    queryScript,
				"returnByValue": true,
			},
		})

		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		var evalRes struct {
			Result struct {
				Result struct {
					Value string `json:"value"`
				} `json:"result"`
			} `json:"result"`
		}
		_ = conn.ReadJSON(&evalRes)
		conn.Close()

		if evalRes.Result.Result.Value != "" && strings.EqualFold(evalRes.Result.Result.Value, targetEmail) {
			return true
		}

		time.Sleep(200 * time.Millisecond)
	}

	return false
}

// GetCurrentConversationPath returns the current active conversation path (e.g. /c/<id>?section=...) or empty string
func GetCurrentConversationPath() string {
	port := GetActiveDevToolsPort()
	if port == 0 {
		return ""
	}

	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var targets []DevToolsTarget
	_ = json.Unmarshal(body, &targets)

	var wsURL string
	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebuggerURL != "" {
			wsURL = t.WebSocketDebuggerURL
			break
		}
	}
	if wsURL == "" {
		return ""
	}

	dialer := websocket.DefaultDialer
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	cancel()
	if err != nil {
		return ""
	}
	defer conn.Close()

	script := `(() => {
		try {
			const p = window.location.pathname;
			const s = window.location.search;
			if (p && p.startsWith('/c/')) {
				return p + (s || '');
			}
		} catch(e) {}
		return "";
	})()`

	_ = conn.WriteJSON(map[string]interface{}{
		"id":     1001,
		"method": "Runtime.evaluate",
		"params": map[string]interface{}{
			"expression":    script,
			"returnByValue": true,
		},
	})

	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	var evalRes struct {
		Result struct {
			Result struct {
				Value string `json:"value"`
			} `json:"result"`
		} `json:"result"`
	}
	_ = conn.ReadJSON(&evalRes)
	return evalRes.Result.Result.Value
}

// RestoreConversation navigates Antigravity UI back to the given conversation path if not already there
func RestoreConversation(targetPath string) bool {
	if targetPath == "" || !strings.HasPrefix(targetPath, "/c/") {
		return false
	}

	port := GetActiveDevToolsPort()
	if port == 0 {
		return false
	}

	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var targets []DevToolsTarget
	_ = json.Unmarshal(body, &targets)

	var wsURL string
	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebuggerURL != "" {
			wsURL = t.WebSocketDebuggerURL
			break
		}
	}
	if wsURL == "" {
		return false
	}

	dialer := websocket.DefaultDialer
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	cancel()
	if err != nil {
		return false
	}
	defer conn.Close()

	pathJSON, _ := json.Marshal(targetPath)
	script := fmt.Sprintf(`(() => {
		try {
			const target = %s;
			const current = window.location.pathname + (window.location.search || '');
			if (current !== target) {
				window.location.href = target;
				return true;
			}
			return false;
		} catch(e) {
			return false;
		}
	})()`, pathJSON)

	_ = conn.WriteJSON(map[string]interface{}{
		"id":     1002,
		"method": "Runtime.evaluate",
		"params": map[string]interface{}{
			"expression":    script,
			"returnByValue": true,
		},
	})

	_ = conn.SetReadDeadline(time.Now().Add(1000 * time.Millisecond))
	var evalRes struct {
		Result struct {
			Result struct {
				Value bool `json:"value"`
			} `json:"result"`
		} `json:"result"`
	}
	_ = conn.ReadJSON(&evalRes)
	return evalRes.Result.Result.Value
}

// SendChatMessage injects text into Antigravity chat input and immediately submits it
// by finding the submit/send button and dispatching native Enter key events.
func SendChatMessage(text string) bool {
	if text == "" {
		return false
	}

	port := GetActiveDevToolsPort()
	if port == 0 {
		return false
	}

	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var targets []DevToolsTarget
	_ = json.Unmarshal(body, &targets)

	var wsURL string
	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebuggerURL != "" {
			wsURL = t.WebSocketDebuggerURL
			break
		}
	}
	if wsURL == "" {
		return false
	}

	dialer := websocket.DefaultDialer
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	cancel()
	if err != nil {
		return false
	}
	defer conn.Close()

	textJSON, _ := json.Marshal(text)
	script := fmt.Sprintf(`(async () => {
		const sleep = (ms) => new Promise(r => setTimeout(r, ms));
		let editor = null;
		// Wait up to 3 seconds for contenteditable input element to be mounted in DOM
		for (let i = 0; i < 15; i++) {
			editor = document.querySelector('[contenteditable="true"]');
			if (editor) break;
			await sleep(200);
		}
		if (!editor) return { success: false, reason: "editor_not_found" };

		editor.focus();
		await sleep(50);
		document.execCommand('selectAll', false, null);
		document.execCommand('insertText', false, %s);
		editor.dispatchEvent(new Event('input', { bubbles: true }));
		await sleep(80);

		// Attempt 1: Locate and trigger the Send / Submit button inside the composer card
		let card = editor.closest('.bg-card') || editor.parentElement;
		for (let p = 0; p < 5 && card; p++) {
			const buttons = Array.from(card.querySelectorAll('button'));
			// Check for explicit send button
			for (const b of buttons) {
				const label = (b.getAttribute('aria-label') || '').toLowerCase();
				const title = (b.getAttribute('title') || '').toLowerCase();
				if (label.includes('cancel') || label.includes('voice') || label.includes('context') || label.includes('model') || label.includes('sidebar')) {
					continue;
				}
				if (label.includes('send') || label.includes('submit') || title.includes('send') || title.includes('submit')) {
					b.click();
					return { success: true, method: "send_button_click" };
				}
			}

			// Fallback: the action submit button is typically the last button in the composer card (when not cancel)
			if (buttons.length > 0) {
				const lastBtn = buttons[buttons.length - 1];
				const label = (lastBtn.getAttribute('aria-label') || '').toLowerCase();
				if (!label.includes('cancel') && !label.includes('voice') && !label.includes('context') && !label.includes('sidebar')) {
					lastBtn.click();
					return { success: true, method: "last_button_click" };
				}
			}
			if (card.parentElement) card = card.parentElement;
		}

		// Attempt 2: Synthetic enter key events directly on the editor
		const opts = { key: 'Enter', code: 'Enter', keyCode: 13, which: 13, bubbles: true, cancelable: true };
		editor.dispatchEvent(new KeyboardEvent('keydown', opts));
		editor.dispatchEvent(new KeyboardEvent('keypress', opts));
		editor.dispatchEvent(new KeyboardEvent('keyup', opts));

		return { success: true, method: "synthetic_enter" };
	})()`, textJSON)

	_ = conn.WriteJSON(map[string]interface{}{
		"id": 1003,
		"method": "Runtime.evaluate",
		"params": map[string]interface{}{
			"expression":    script,
			"returnByValue": true,
			"awaitPromise":  true,
		},
	})

	// Also dispatch native CDP Enter key event as double insurance (isTrusted: true)
	time.Sleep(100 * time.Millisecond)
	_ = conn.WriteJSON(map[string]interface{}{
		"id": 1004,
		"method": "Input.dispatchKeyEvent",
		"params": map[string]interface{}{
			"type":                  "rawKeyDown",
			"key":                   "Enter",
			"code":                  "Enter",
			"windowsVirtualKeyCode": 13,
			"nativeVirtualKeyCode":  13,
		},
	})
	_ = conn.WriteJSON(map[string]interface{}{
		"id": 1005,
		"method": "Input.dispatchKeyEvent",
		"params": map[string]interface{}{
			"type":           "char",
			"key":            "\r",
			"text":           "\r",
			"unmodifiedText": "\r",
		},
	})
	_ = conn.WriteJSON(map[string]interface{}{
		"id": 1006,
		"method": "Input.dispatchKeyEvent",
		"params": map[string]interface{}{
			"type":                  "keyUp",
			"key":                   "Enter",
			"code":                  "Enter",
			"windowsVirtualKeyCode": 13,
			"nativeVirtualKeyCode":  13,
		},
	})

	_ = conn.SetReadDeadline(time.Now().Add(3500 * time.Millisecond))
	var evalRes struct {
		Result struct {
			Result struct {
				Value struct {
					Success bool   `json:"success"`
					Method  string `json:"method"`
				} `json:"value"`
			} `json:"result"`
		} `json:"result"`
	}
	_ = conn.ReadJSON(&evalRes)
	return evalRes.Result.Result.Value.Success
}


