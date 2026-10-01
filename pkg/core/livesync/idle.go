package livesync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"agy-tools/pkg/core/process"
	"github.com/gorilla/websocket"
)

type IdleStatus struct {
	IsRunning bool   `json:"isRunning"` // Is Antigravity IDE process running
	IsIdle    bool   `json:"isIdle"`    // Is Antigravity IDE idle (not generating or executing)
	Reason    string `json:"reason"`    // Reason (e.g., "idle", "ide_not_running", "executing_task", "generating_response")
}

// CheckAntigravityIdle checks whether Antigravity IDE is currently idle or generating/executing
func CheckAntigravityIdle() (IdleStatus, error) {
	// 1. Process check: if Antigravity is not running, it is completely idle and safe to switch
	procs, err := process.GetActiveProcesses()
	if err == nil {
		hasAntigravity := false
		for _, p := range procs {
			if strings.EqualFold(p.Name, "antigravity.exe") {
				hasAntigravity = true
				break
			}
		}
		if !hasAntigravity {
			return IdleStatus{
				IsRunning: false,
				IsIdle:    true,
				Reason:    "ide_not_running",
			}, nil
		}
	}

	port := GetActiveDevToolsPort()
	if port == 0 {
		return IdleStatus{
			IsRunning: false,
			IsIdle:    true,
			Reason:    "ide_not_running",
		}, nil
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
	if err != nil {
		// Double check if process is still running
		procs, pErr := process.GetActiveProcesses()
		if pErr == nil {
			hasAntigravity := false
			for _, p := range procs {
				if strings.EqualFold(p.Name, "antigravity.exe") {
					hasAntigravity = true
					break
				}
			}
			if !hasAntigravity {
				return IdleStatus{
					IsRunning: false,
					IsIdle:    true,
					Reason:    "ide_not_running",
				}, nil
			}
		}
		return IdleStatus{
			IsRunning: true,
			IsIdle:    false,
			Reason:    "devtools_unreachable",
		}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return IdleStatus{
			IsRunning: true,
			IsIdle:    false,
			Reason:    "devtools_read_error",
		}, err
	}

	var targets []DevToolsTarget
	if err := json.Unmarshal(body, &targets); err != nil {
		return IdleStatus{
			IsRunning: true,
			IsIdle:    false,
			Reason:    "devtools_json_error",
		}, err
	}

	var pageTarget *DevToolsTarget
	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebuggerURL != "" {
			pageTarget = &t
			break
		}
	}

	if pageTarget == nil {
		return IdleStatus{
			IsRunning: true,
			IsIdle:    true,
			Reason:    "no_page_target",
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, pageTarget.WebSocketDebuggerURL, nil)
	if err != nil {
		return IdleStatus{
			IsRunning: true,
			IsIdle:    false,
			Reason:    "devtools_ws_dial_failed",
		}, err
	}
	defer conn.Close()

	script := `(() => {
		// 1. Check DOM for Stop or Cancel buttons
		const stopBtn = document.querySelector('button[aria-label*="Stop execution"], button[aria-label*="Cancel (Ctrl+D)"], button[aria-label*="Stop generating"], button[aria-label*="Cancel task"], button[aria-label="Stop"], button[aria-label="stop"], button[aria-label*="Stop"], button[aria-label*="Cancel"]');
		if (stopBtn) {
			const label = stopBtn.getAttribute("aria-label") || stopBtn.innerText || "stop_button_active";
			return { isIdle: false, reason: "stop_button_active (" + label.trim().slice(0, 30) + ")" };
		}

		// 2. Check React fiber tree for active running/streaming flags
		const root = document.getElementById("root") || document.body.firstElementChild;
		const key = Object.keys(root || {}).find(k => k.startsWith("__reactContainer"));
		let queue = root && key ? [root[key]] : [];
		while (queue.length > 0 && queue.length < 5000) {
			const f = queue.shift();
			if (!f) continue;
			const p = f.memoizedProps;
			if (p) {
				if (p.isRunning === true) return { isIdle: false, reason: "task_executing" };
				if (p.isStreaming === true) return { isIdle: false, reason: "response_streaming" };
				if (p.isGenerating === true) return { isIdle: false, reason: "model_generating" };
			}
			if (f.child) queue.push(f.child);
			if (f.sibling) queue.push(f.sibling);
		}

		return { isIdle: true, reason: "idle" };
	})()`

	payload := map[string]interface{}{
		"id":     1,
		"method": "Runtime.evaluate",
		"params": map[string]interface{}{
			"expression":    script,
			"returnByValue": true,
		},
	}

	if err := conn.WriteJSON(payload); err != nil {
		return IdleStatus{
			IsRunning: true,
			IsIdle:    false,
			Reason:    "devtools_eval_write_failed",
		}, err
	}

	_ = conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
	var evalResp struct {
		Result struct {
			Result struct {
				Value struct {
					IsIdle bool   `json:"isIdle"`
					Reason string `json:"reason"`
				} `json:"value"`
			} `json:"result"`
		} `json:"result"`
	}

	if err := conn.ReadJSON(&evalResp); err != nil {
		return IdleStatus{
			IsRunning: true,
			IsIdle:    false,
			Reason:    "devtools_eval_read_failed",
		}, err
	}

	val := evalResp.Result.Result.Value
	return IdleStatus{
		IsRunning: true,
		IsIdle:    val.IsIdle,
		Reason:    val.Reason,
	}, nil
}
