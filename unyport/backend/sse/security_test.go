package sse

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNftablesRuntimeStateDetectsLoadedRuleset(t *testing.T) {
	dir := t.TempDir()
	nft := filepath.Join(dir, "nft")
	if err := os.WriteFile(nft, []byte("#!/bin/sh\nprintf 'table inet filter {}\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	installed, active := nftablesRuntimeState()
	if !installed || !active {
		t.Fatalf("expected installed active nftables, got installed=%v active=%v", installed, active)
	}
}

func TestNftablesRuntimeStateRejectsEmptyRuleset(t *testing.T) {
	dir := t.TempDir()
	nft := filepath.Join(dir, "nft")
	if err := os.WriteFile(nft, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	installed, active := nftablesRuntimeState()
	if !installed || active {
		t.Fatalf("expected installed inactive nftables, got installed=%v active=%v", installed, active)
	}
}

func TestSecurityProcessTargetsHideAbsentDaemonsAndPreferSSHD(t *testing.T) {
	targets := []string{"unyport", "sshd", "dropbear", "xenstored", "dnsmasq"}
	found := map[string][]int{"unyport": {12}, "dnsmasq": {42}}
	installed := func(name string) bool { return name == "sshd" || name == "dropbear" }
	got := selectSecurityProcessTargets(targets, found, installed)
	want := []string{"unyport", "sshd", "dnsmasq"}
	if len(got) != len(want) {
		t.Fatalf("unexpected targets: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected targets: got %v want %v", got, want)
		}
	}
}

func TestTCPListenersMergeAddressFamiliesByPort(t *testing.T) {
	got := mergeTCPListeners([]SecurityListener{
		{Proto: "tcp4", Port: 443, Status: "warn"},
		{Proto: "tcp6", Port: 443, Status: "warn"},
		{Proto: "tcp6", Port: 8800, Status: "ok"},
	})
	if len(got) != 2 {
		t.Fatalf("expected two unique ports, got %v", got)
	}
	for _, listener := range got {
		if listener.Port == 443 && listener.Proto != "tcp4/6" {
			t.Fatalf("expected merged protocol, got %q", listener.Proto)
		}
	}
}

func TestPort1024IsNotPrivileged(t *testing.T) {
	check := listenerExposureCheck([]SecurityListener{{Proto: "tcp4", Port: 1024, Status: "ok"}})
	if check.Status != "ok" {
		t.Fatalf("port 1024 must be unprivileged, got %q", check.Status)
	}
}
