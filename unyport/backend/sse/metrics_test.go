package sse

import "testing"

func TestNaturalLessOrdersDiskDevices(t *testing.T) {
	devices := []string{"/dev/sdb1", "/dev/sda10", "/dev/sda2", "/dev/sda1", "/dev/nvme0n1p10", "/dev/nvme0n1p2"}
	want := []string{"/dev/nvme0n1p2", "/dev/nvme0n1p10", "/dev/sda1", "/dev/sda2", "/dev/sda10", "/dev/sdb1"}

	for i := 0; i < len(devices); i++ {
		for j := i + 1; j < len(devices); j++ {
			if naturalLess(devices[j], devices[i]) {
				devices[i], devices[j] = devices[j], devices[i]
			}
		}
	}

	for i := range want {
		if devices[i] != want[i] {
			t.Fatalf("devices[%d] = %q, want %q; got %v", i, devices[i], want[i], devices)
		}
	}
}
