package util

import (
	"os"
	"os/user"
)

// UserHomeDir returns the home directory for the current user,
// taking into account sudo context.
func UserHomeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if sudo := os.Getenv("SUDO_USER"); sudo != "" {
		if u, err := user.Lookup(sudo); err == nil {
			return u.HomeDir
		}
	}
	home, _ := os.UserHomeDir()
	return home
}

// SudoUserInfo returns user info for the original user when running under sudo.
func SudoUserInfo() *user.User {
	sudo := os.Getenv("SUDO_USER")
	if sudo != "" {
		if u, err := user.Lookup(sudo); err == nil {
			return u
		}
	}
	return nil
}