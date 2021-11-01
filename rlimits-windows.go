//go:build (!linux && !freebsd) || windows
// +build !linux,!freebsd windows

package main

// @rlimits-windows.go contains empty function to allow compilation on on unix-like OSes.
// It real implementation is done into the file named <rlimits-linux.go> for unix-like OSes.
// enforceMaxOpenFiles is used for unix-like (linux and freebsd) platforms only.
func enforceMaxOpenFiles() {
	// ignore
}
