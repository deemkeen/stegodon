package util

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	AppConfigDir = ".config/stegodon"
)

// GetConfigDir returns the stegodon config directory path (~/.config/stegodon/)
// and creates it if it doesn't exist
func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, AppConfigDir)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return configDir, nil
}

// ResolveFilePath resolves a file path with the following priority:
// 1. Local working directory (e.g., ./database.db)
// 2. User config directory (e.g., ~/.config/stegodon/database.db)
// 3. Returns the user config directory path if neither exists (for creation)
func ResolveFilePath(filename string) string {
	// Check local directory first
	if _, err := os.Stat(filename); err == nil {
		return filename
	}

	// Try user config directory
	configDir, err := GetConfigDir()
	if err != nil {
		// Fallback to local directory if we can't get config dir
		return filename
	}

	userPath := filepath.Join(configDir, filename)

	// If file exists in user dir, return that path
	if _, err := os.Stat(userPath); err == nil {
		return userPath
	}

	// Neither exists, return user config path (for creation)
	return userPath
}

// ResolveFilePathWithSubdir resolves a file path in a subdirectory
// Priority:
// 1. Local working directory (e.g., ./.ssh/stegodonhostkey)
// 2. User config directory (e.g., ~/.config/stegodon/.ssh/stegodonhostkey)
// 3. Returns the user config directory path if neither exists (for creation)
func ResolveFilePathWithSubdir(subdir, filename string) string {
	localPath := filepath.Join(subdir, filename)

	// Check local directory first
	if _, err := os.Stat(localPath); err == nil {
		return localPath
	}

	// Try user config directory
	configDir, err := GetConfigDir()
	if err != nil {
		// Fallback to local directory if we can't get config dir
		return localPath
	}

	userSubdir := filepath.Join(configDir, subdir)
	userPath := filepath.Join(userSubdir, filename)

	// If file exists in user dir, return that path
	if _, err := os.Stat(userPath); err == nil {
		return userPath
	}

	// Neither exists, create subdirectory and return user config path
	os.MkdirAll(userSubdir, 0755)
	return userPath
}
