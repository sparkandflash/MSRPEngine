package vron

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	promptCache sync.Map // filename -> string, lazy-loaded and cached
)

// resolvePromptsDir returns the absolute path to the src/prompts directory.
// It tries the binary's directory first, then falls back to the working directory.
func resolvePromptsDir() string {
	exePath, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exePath), "src", "prompts")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join("src", "prompts")
}

// loadPrompt reads a prompt from the prompts directory by filename.
// Results are cached in memory after the first load so disk is only hit once per session.
func loadPrompt(filename string) string {
	if cached, ok := promptCache.Load(filename); ok {
		return cached.(string)
	}

	path := filepath.Join(resolvePromptsDir(), filename)
	data, err := os.ReadFile(path)
	if err != nil {
		// Fallback string if file cannot be read
		return fmt.Sprintf("Execute method %s strictly.", filename)
	}

	content := string(data)
	promptCache.Store(filename, content)
	return content
}

// GetMethodPrompt loads the prompt specific to the VRon's assigned method.
func GetMethodPrompt(method Method) string {
	filename := fmt.Sprintf("method_%s.txt", string(method))
	return loadPrompt(filename)
}

// GetVRonIdentityPrompt loads the master system prompt for default/legacy paths.
func GetVRonIdentityPrompt() string {
	return loadPrompt("vron_identity.txt")
}

// GetMasterVRonPrompt is the canonical entry point used by the engine.
func GetMasterVRonPrompt() string {
	return GetVRonIdentityPrompt()
}
