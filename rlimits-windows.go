//go:build windows
// +build windows

package main

// @rlimits-windows contains empty function to allow compilation on windows platforms.
// It real implementation is done into the file named <rlimits-linuxgo> for linux.
// enforceMaxOpenFiles is used for linux-based platforms only.
func enforceMaxOpenFiles() {
	// ignore
}
