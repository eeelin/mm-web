package server

import "testing"

func TestCurrentBuildInfoHasRuntimeDetails(t *testing.T) {
	info := currentBuildInfo()
	if info.Version == "" || info.GoVersion == "" || info.OS == "" || info.Arch == "" {
		t.Fatalf("currentBuildInfo() = %#v", info)
	}
}

func TestShortRevision(t *testing.T) {
	if got := shortRevision("1234567890abcdef"); got != "12345678" {
		t.Fatalf("shortRevision() = %q", got)
	}
}
