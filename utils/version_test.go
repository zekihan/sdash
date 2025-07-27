package utils

import (
	"testing"
)

func TestGetStartupInfo_Clean(t *testing.T) {
	// Save and restore globals.
	origV, origD, origC, origG := Version, BuildDate, CommitHash, GitTreeState

	defer func() {
		Version, BuildDate, CommitHash, GitTreeState = origV, origD, origC, origG
	}()

	Version = "1.2.3"
	BuildDate = "2023-01-02"
	CommitHash = "abcd123"
	GitTreeState = "clean"

	got := GetStartupInfo()

	want := "Version: 1.2.3, Build Date: 2023-01-02, CommitHash: abcd123"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGetStartupInfo_Dirty(t *testing.T) {
	// Save and restore globals.
	origV, origD, origC, origG := Version, BuildDate, CommitHash, GitTreeState

	defer func() {
		Version, BuildDate, CommitHash, GitTreeState = origV, origD, origC, origG
	}()

	Version = "2.0.0"
	BuildDate = "2024-06-07"
	CommitHash = "deadbeef"
	GitTreeState = "dirty"

	got := GetStartupInfo()

	want := "Version: 2.0.0, Build Date: 2024-06-07, CommitHash: deadbeef (dirty)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
