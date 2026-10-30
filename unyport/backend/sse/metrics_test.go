package sse

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSelectMainIfacePrefersBridgeWithIPv4(t *testing.T) {
	rx := map[string]uint64{
		"eth0":   9000,
		"br-wan": 8000,
		"br-srv": 100,
	}
	ips := map[string]string{
		"br-wan": "192.168.4.2",
	}

	if got := selectMainIface(rx, ips); got != "br-wan" {
		t.Fatalf("selectMainIface() = %q, want br-wan", got)
	}
}

func TestSelectMainIfaceFallsBackToTrafficWithoutIPv4(t *testing.T) {
	rx := map[string]uint64{"eth0": 9000, "br-srv": 100}

	if got := selectMainIface(rx, nil); got != "eth0" {
		t.Fatalf("selectMainIface() = %q, want eth0", got)
	}
}

func TestApplyInterfaceMastersExposesBridgeAndItsIP(t *testing.T) {
	ifaces := []NetIfaceInfo{{Name: "eth0"}, {Name: "vif3.0"}}
	masters := map[string]string{"eth0": "br-wan", "vif3.0": "br-srv"}
	ips := map[string]string{"br-wan": "192.168.4.2"}

	applyInterfaceMasters(ifaces, masters, ips)

	if ifaces[0].Master != "br-wan" || ifaces[0].MasterIP != "192.168.4.2" {
		t.Fatalf("eth0 topology = master %q ip %q", ifaces[0].Master, ifaces[0].MasterIP)
	}
	if ifaces[1].Master != "br-srv" || ifaces[1].MasterIP != "" {
		t.Fatalf("vif topology = master %q ip %q", ifaces[1].Master, ifaces[1].MasterIP)
	}
}

func TestDeviceTypeForNeighborAlwaysReturnsAType(t *testing.T) {
	tests := []struct {
		name, ip, mac, iface, inventory, want string
	}{
		{"explicit host", "192.168.4.1", "14:84:30:0f:be:99", "br-wan", "192.168.4.1=server", "server"},
		{"explicit printer", "192.168.4.20", "aa:bb:cc:dd:ee:ff", "br-wan", "192.168.4.20=printer", "printer"},
		{"explicit BLE device", "192.168.4.21", "aa:bb:cc:dd:ee:01", "br-wan", "192.168.4.21=ble", "ble"},
		{"explicit 3D printer", "192.168.4.22", "aa:bb:cc:dd:ee:02", "br-wan", "192.168.4.22=3d_printer", "3d_printer"},
		{"storage type rejected on network", "192.168.4.23", "aa:bb:cc:dd:ee:03", "br-wan", "192.168.4.23=hdd_35", "unknown"},
		{"NAS accepted on network", "192.168.4.25", "aa:bb:cc:dd:ee:05", "br-wan", "192.168.4.25=nas", "nas"},
		{"SAN accepted on network", "192.168.4.26", "aa:bb:cc:dd:ee:06", "br-wan", "192.168.4.26=san", "san"},
		{"explicit network NAS", "192.168.4.24", "aa:bb:cc:dd:ee:04", "br-wan", "192.168.4.24=network_nas", "network_nas"},
		{"xen guest", "192.168.4.3", "00:16:3e:61:75:64", "br-wan", "", "server"},
		{"unidentified", "192.168.4.99", "aa:bb:cc:dd:ee:ff", "br-wan", "", "unknown"},
		{"invalid explicit type", "192.168.4.99", "aa:bb:cc:dd:ee:ff", "br-wan", "192.168.4.99=spaceship", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deviceTypeForNeighbor(tt.ip, tt.mac, tt.iface, tt.inventory); got != tt.want {
				t.Fatalf("deviceTypeForNeighbor() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeviceTypeForHostname(t *testing.T) {
	tests := map[string]string{
		"HISENSE-TV":          "smart_tv",
		"xiaomi-13t-pro":      "phone",
		"WINDOWS-WORKSTATION": "desktop",
		"TRINITY-INSTALL":     "desktop",
		"unknown-client":      "unknown",
	}
	for hostname, want := range tests {
		if got := deviceTypeForHostname(hostname); got != want {
			t.Errorf("deviceTypeForHostname(%q) = %q, want %q", hostname, got, want)
		}
	}
}

func TestManualDeviceTypeOverridesDHCPHostname(t *testing.T) {
	got, source := deviceTypeForNeighborDetected("192.168.2.3", "a0:62:fb:1e:ab:23", "br-lan", "192.168.2.3=server", "HISENSE-TV")
	if got != "server" || source != "manual" {
		t.Fatalf("type/source = %q/%q, want server/manual", got, source)
	}
}

func TestPublicNeighborUsesWANIconType(t *testing.T) {
	tests := []struct {
		ip   string
		want string
	}{
		{"93.25.54.1", "router_wan"},
		{"192.168.2.3", "unknown"},
		{"10.0.0.1", "unknown"},
		{"100.64.0.1", "unknown"},
		{"169.254.1.1", "unknown"},
		{"127.0.0.1", "unknown"},
	}
	for _, tt := range tests {
		got, _ := deviceTypeForNeighborDetected(tt.ip, "aa:bb:cc:dd:ee:ff", "eth0", "", "")
		if got != tt.want {
			t.Errorf("device type for %s = %q, want %q", tt.ip, got, tt.want)
		}
	}
}

func TestReadKeaLeaseMetadataUsesNumberedActiveFileAndSkipsBackups(t *testing.T) {
	dir := t.TempDir()
	header := "address,hwaddr,client_id,valid_lifetime,expire,subnet_id,fqdn_fwd,fqdn_rev,hostname,state,user_context,pool_id\n"
	if err := os.WriteFile(filepath.Join(dir, "dhcp4.leases"), []byte(header), 0600); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(dir, "dhcp4.leases.2")
	if err := os.WriteFile(active, []byte(header+"192.168.2.101,e4:bc:aa:ea:64:52,,86400,0,1,0,0,xiaomi-13t-pro,0,,0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(dir, "dhcp4.leases.2.bak-old")
	if err := os.WriteFile(backup, []byte(header+"192.168.2.101,00:00:00:00:00:01,,86400,0,1,0,0,wrong-host,0,,0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	_ = os.Chtimes(active, now, now)
	_ = os.Chtimes(backup, now.Add(time.Hour), now.Add(time.Hour))

	metadata := make(map[string]neighborMetadata)
	readKeaLeaseMetadata(metadata, dir)
	meta := metadataForNeighbor(metadata, "192.168.2.101", "e4:bc:aa:ea:64:52")
	if meta.Hostname != "xiaomi-13t-pro" {
		t.Fatalf("hostname = %q, want xiaomi-13t-pro", meta.Hostname)
	}
}

func TestReadKeaReservationMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kea-dhcp4.conf")
	config := `{"Dhcp4":{"subnet4":[{"reservations":[{"hw-address":"A0:62:FB:1E:AB:23","ip-address":"192.168.2.3","hostname":"HISENSE-TV"}]}]}}`
	if err := os.WriteFile(path, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	metadata := make(map[string]neighborMetadata)
	readKeaReservationMetadata(metadata, path)
	meta := metadataForNeighbor(metadata, "192.168.2.3", "a0:62:fb:1e:ab:23")
	if meta.Hostname != "HISENSE-TV" {
		t.Fatalf("hostname = %q, want HISENSE-TV", meta.Hostname)
	}
}

func TestReadHostsMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(path, []byte("127.0.0.1 localhost\n192.168.2.100 TRINITY-INSTALL install\n"), 0600); err != nil {
		t.Fatal(err)
	}
	metadata := make(map[string]neighborMetadata)
	readHostsMetadata(metadata, path)
	if got := metadataForNeighbor(metadata, "192.168.2.100", "").Hostname; got != "TRINITY-INSTALL" {
		t.Fatalf("hostname = %q, want TRINITY-INSTALL", got)
	}
}

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

func TestRootBlockDevice(t *testing.T) {
	tests := map[string]string{
		"/dev/sda1": "sda", "/dev/xvda2": "xvda", "/dev/vda3": "vda",
		"/dev/nvme0n1p2": "nvme0n1", "/dev/mmcblk0p1": "mmcblk0",
	}
	for device, want := range tests {
		if got := rootBlockDevice(device); got != want {
			t.Errorf("rootBlockDevice(%q) = %q, want %q", device, got, want)
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
