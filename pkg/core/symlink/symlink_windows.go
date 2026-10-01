package symlink

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"agy-tools/pkg/config"
	"agy-tools/pkg/types"
)

type SharedLinkDef struct {
	ID          string
	Origin      string
	Destination string
}

func GetSharedLinkDefinitions() []SharedLinkDef {
	return []SharedLinkDef{
		{
			ID:          "conversations:primary",
			Origin:      config.ConversationsPrimary,
			Destination: config.SharedConversationsDir,
		},
		{
			ID:          "conversations:ide",
			Origin:      config.ConversationsIDE,
			Destination: config.SharedConversationsDir,
		},
		{
			ID:          "skills:config",
			Origin:      config.SkillsConfig,
			Destination: config.SharedSkillsDir,
		},
		{
			ID:          "skills:antigravity",
			Origin:      config.SkillsAntigravity,
			Destination: config.SharedSkillsDir,
		},
	}
}

// IsSymlinkOrJunction checks if the given path is a symlink or directory junction
func IsSymlinkOrJunction(path string) (bool, string, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "", nil
		}
		return false, "", err
	}

	// In Go on Windows, ModeSymlink or ModeIrregular is set for symlinks and junctions
	if fi.Mode()&os.ModeSymlink != 0 || fi.Mode()&os.ModeIrregular != 0 {
		target, err := os.Readlink(path)
		if err == nil {
			return true, target, nil
		}
		return true, "", nil
	}

	return false, "", nil
}

// copyDir recursively copies files from src to dst if dst does not have them
func copyDir(src, dst string) error {
	_ = os.MkdirAll(dst, 0755)
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			_ = copyDir(srcPath, dstPath)
		} else {
			if _, err := os.Stat(dstPath); os.IsNotExist(err) {
				in, err := os.Open(srcPath)
				if err == nil {
					out, err := os.Create(dstPath)
					if err == nil {
						_, _ = io.Copy(out, in)
						_ = out.Close()
					}
					_ = in.Close()
				}
			}
		}
	}
	return nil
}

// EnsureSharedLink verifies and creates the shared link
func EnsureSharedLink(def SharedLinkDef) error {
	// 1. Ensure destination directory exists
	if err := os.MkdirAll(def.Destination, 0755); err != nil {
		return fmt.Errorf("failed to create destination dir %s: %w", def.Destination, err)
	}

	// 2. Ensure parent of origin exists
	parent := filepath.Dir(def.Origin)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("failed to create parent dir %s: %w", parent, err)
	}

	// 3. Check current state of origin
	isLink, target, err := IsSymlinkOrJunction(def.Origin)
	if err == nil && isLink {
		// Clean and compare targets
		if target != "" {
			cleanTarget := filepath.Clean(target)
			cleanDest := filepath.Clean(def.Destination)
			if strings.EqualFold(cleanTarget, cleanDest) {
				return nil // Already correctly linked
			}
		}
		// Pointing to wrong destination, remove link
		_ = os.Remove(def.Origin)
	} else if err == nil {
		fi, statErr := os.Stat(def.Origin)
		if statErr == nil && fi.IsDir() {
			// Real directory with potential contents: migrate first
			_ = copyDir(def.Origin, def.Destination)
			_ = os.RemoveAll(def.Origin)
		}
	}

	// 4. Create Directory Junction using mklink /J (does not require Admin privileges on Windows)
	cmd := exec.Command("cmd", "/c", "mklink", "/J", def.Origin, def.Destination)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Fallback to os.Symlink
		if symErr := os.Symlink(def.Destination, def.Origin); symErr != nil {
			return fmt.Errorf("mklink failed: %s (%v), symlink failed: %w", string(out), err, symErr)
		}
	}

	return nil
}

// EnsureAllSharedLinks ensures all 4 shared symlinks exist and point to shared dir
func EnsureAllSharedLinks() []types.DiagnosticItem {
	var items []types.DiagnosticItem
	defs := GetSharedLinkDefinitions()

	for _, def := range defs {
		err := EnsureSharedLink(def)
		if err != nil {
			items = append(items, types.DiagnosticItem{
				Category: "Shared Symlinks",
				Name:     def.ID,
				Status:   "FAIL",
				Message:  fmt.Sprintf("Failed to link %s -> %s", def.Origin, def.Destination),
				Details:  err.Error(),
				Fix:      "Run with administrative permissions or ensure Developer Mode is enabled",
			})
		} else {
			items = append(items, types.DiagnosticItem{
				Category: "Shared Symlinks",
				Name:     def.ID,
				Status:   "OK",
				Message:  fmt.Sprintf("%s -> %s", def.Origin, def.Destination),
			})
		}
	}

	return items
}

// VerifyAllSharedLinks checks the health of all shared symlinks
func VerifyAllSharedLinks() []types.DiagnosticItem {
	var items []types.DiagnosticItem
	defs := GetSharedLinkDefinitions()

	for _, def := range defs {
		if _, err := os.Stat(def.Destination); os.IsNotExist(err) {
			items = append(items, types.DiagnosticItem{
				Category: "Shared Symlinks",
				Name:     def.ID,
				Status:   "WARN",
				Message:  fmt.Sprintf("Shared target does not exist: %s", def.Destination),
				Fix:      "Run 'agy-tools doctor --fix' to create shared directory",
			})
			continue
		}

		isLink, target, err := IsSymlinkOrJunction(def.Origin)
		if err != nil || !isLink {
			items = append(items, types.DiagnosticItem{
				Category: "Shared Symlinks",
				Name:     def.ID,
				Status:   "WARN",
				Message:  fmt.Sprintf("Origin is not a symlink: %s", def.Origin),
				Fix:      "Run 'agy-tools doctor --fix' to link to shared directory",
			})
		} else {
			items = append(items, types.DiagnosticItem{
				Category: "Shared Symlinks",
				Name:     def.ID,
				Status:   "OK",
				Message:  fmt.Sprintf("%s -> %s", def.Origin, target),
			})
		}
	}

	return items
}
