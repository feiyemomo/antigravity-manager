package process

import (
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"agy-tools/pkg/types"
	"golang.org/x/sys/windows"
)

var monitoredNames = []string{
	"antigravity.exe",
	"language_server.exe",
	"agy.exe",
	"agy",
}

// GetActiveProcesses returns all running processes matching Antigravity or agy
func GetActiveProcesses() ([]types.ProcessInfo, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to create process snapshot: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil, fmt.Errorf("failed to get first process: %w", err)
	}

	var found []types.ProcessInfo

	for {
		name := syscall.UTF16ToString(entry.ExeFile[:])
		lowerName := strings.ToLower(name)

		for _, m := range monitoredNames {
			if lowerName == m {
				found = append(found, types.ProcessInfo{
					PID:  entry.ProcessID,
					Name: name,
				})
				break
			}
		}

		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}

	return found, nil
}

// CheckProcessSafety verifies that no Antigravity processes are running, unless skipped
func CheckProcessSafety(skipCheck bool) error {
	if skipCheck {
		return nil
	}

	procs, err := GetActiveProcesses()
	if err != nil {
		return nil // Non-fatal if process enumeration fails
	}

	if len(procs) > 0 {
		var lines []string
		for _, p := range procs {
			lines = append(lines, fmt.Sprintf("%s (PID: %d)", p.Name, p.PID))
		}
		return fmt.Errorf("[PROCESS SAFETY VIOLATION] Cannot perform operation while Antigravity is running.\nDetected active process(es):\n  - %s\nPlease completely exit Antigravity IDE and close any running 'agy' CLI sessions, then try again", strings.Join(lines, "\n  - "))
	}

	return nil
}

// RestartLanguageServer terminates language_server.exe so Antigravity's host process restarts it
func RestartLanguageServer() error {
	return RestartLanguageServerAndWait(3 * time.Second)
}

// RestartLanguageServerAndWait terminates language_server.exe and waits until a new instance is spawned and ready
func RestartLanguageServerAndWait(timeout time.Duration) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return err
	}

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := windows.Process32First(snapshot, &entry); err != nil {
		windows.CloseHandle(snapshot)
		return err
	}

	oldPIDs := make(map[uint32]bool)
	for {
		name := strings.ToLower(syscall.UTF16ToString(entry.ExeFile[:]))
		if name == "language_server.exe" {
			oldPIDs[entry.ProcessID] = true
			h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, entry.ProcessID)
			if err == nil {
				_ = windows.TerminateProcess(h, 0)
				_ = windows.CloseHandle(h)
			}
		}

		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}
	windows.CloseHandle(snapshot)

	if len(oldPIDs) == 0 {
		return nil
	}

	// Poll until a new language_server.exe process is detected
	deadline := time.Now().Add(timeout)
	newFound := false
	for time.Now().Before(deadline) {
		time.Sleep(150 * time.Millisecond)

		snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
		if err != nil {
			continue
		}

		var e windows.ProcessEntry32
		e.Size = uint32(unsafe.Sizeof(e))
		if err := windows.Process32First(snap, &e); err == nil {
			for {
				n := strings.ToLower(syscall.UTF16ToString(e.ExeFile[:]))
				if n == "language_server.exe" && !oldPIDs[e.ProcessID] {
					newFound = true
					break
				}
				if err := windows.Process32Next(snap, &e); err != nil {
					break
				}
			}
		}
		windows.CloseHandle(snap)

		if newFound {
			// Give the new process sufficient time to initialize IPC and reload credentials
			time.Sleep(1500 * time.Millisecond)
			break
		}
	}

	return nil
}

