//go:build !linux

package systeminfo

// readPlatform is a no-op where /proc and statfs stats are unavailable.
func readPlatform(_ *Info) {}
