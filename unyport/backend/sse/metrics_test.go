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

func TestComposeHostRoleKeepsContainerAndParentVirtualization(t *testing.T) {
	container := HostRole{Role: "Container", Runtime: "docker", Label: "Container · Docker", Verified: true, Environment: "container", ContainerRuntime: "docker"}
	xenDomU := HostRole{
		Role: "DomU", Runtime: "xen", Label: "Xen DomU · VM Alpine", Verified: true,
		Environment: "vm", Virtualization: "xen", VirtualizationRole: "domu",
	}

	got := composeHostRole(container, true, xenDomU, true)
	if got.Role != "Container" {
		t.Fatalf("role = %q, want Container", got.Role)
	}
	if got.ContainerRuntime != "docker" {
		t.Fatalf("container runtime = %q, want docker", got.ContainerRuntime)
	}
	if got.Virtualization != "xen" || got.VirtualizationRole != "domu" {
		t.Fatalf("virtualization = %q/%q, want xen/domu", got.Virtualization, got.VirtualizationRole)
	}
	if got.ParentRuntime != "xen" {
		t.Fatalf("parent runtime = %q, want xen", got.ParentRuntime)
	}
	if got.Capabilities.XenControl {
		t.Fatal("container inside DomU must not expose Xen control")
	}
	if len(got.Layers) != 2 {
		t.Fatalf("layers len = %d, want 2", len(got.Layers))
	}
}

func TestComposeHostRoleAllowsXenControlOnlyOnBareDom0(t *testing.T) {
	dom0 := HostRole{
		Role: "Dom0", Runtime: "xen", Label: "Xen Dom0 · Hyperviseur", Verified: true,
		Environment: "hypervisor", Virtualization: "xen", VirtualizationRole: "dom0",
	}
	container := HostRole{Role: "Container", Runtime: "docker", Label: "Container · Docker", Verified: true, Environment: "container", ContainerRuntime: "docker"}

	if got := composeHostRole(HostRole{}, false, dom0, true); !got.Capabilities.XenControl {
		t.Fatal("bare Dom0 should expose Xen control")
	}
	if got := composeHostRole(container, true, dom0, true); got.Capabilities.XenControl {
		t.Fatal("container on Dom0 must not expose Xen control")
	}
}

func TestVirtualMachineFromDMIRecognizesQEMUKVM(t *testing.T) {
	got, ok := virtualMachineFromDMI("QEMU Standard PC (Q35 + ICH9, 2009) Red Hat KVM")
	if !ok {
		t.Fatal("expected QEMU/KVM to be detected")
	}
	if got.Role != "DomU" || got.Runtime != "qemu-kvm" || got.VirtualizationRole != "guest" {
		t.Fatalf("got role/runtime/virtRole = %q/%q/%q", got.Role, got.Runtime, got.VirtualizationRole)
	}
}
