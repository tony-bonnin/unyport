package sse

import (
	"encoding/json"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
)

type SecurityStatus struct {
	Summary   SecuritySummary    `json:"summary"`
	Checks    []SecurityCheck    `json:"checks"`
	Services  []SecurityService  `json:"services"`
	Processes []SecurityProcess  `json:"processes"`
	Listeners []SecurityListener `json:"listeners"`
}

type SecuritySummary struct {
	OK       int `json:"ok"`
	Warn     int `json:"warn"`
	Critical int `json:"critical"`
	Unknown  int `json:"unknown"`
}

type SecurityCheck struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Category string `json:"category"`
	Status   string `json:"status"`
	Detail   string `json:"detail"`
}

type SecurityService struct {
	Name      string   `json:"name"`
	State     string   `json:"state"`
	Status    string   `json:"status"`
	Critical  bool     `json:"critical"`
	Runlevels []string `json:"runlevels"`
	Detail    string   `json:"detail"`
}

type SecurityProcess struct {
	Name   string `json:"name"`
	Count  int    `json:"count"`
	Status string `json:"status"`
	PIDs   []int  `json:"pids"`
	Detail string `json:"detail"`
}

type SecurityListener struct {
	Proto  string `json:"proto"`
	Port   int    `json:"port"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

func (b *Broker) SecurityHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(CollectSecurityStatus())
}

func CollectSecurityStatus() SecurityStatus {
	checks := []SecurityCheck{
		sysctlCheck("aslr", "ASLR", "kernel", "/proc/sys/kernel/randomize_va_space", func(v string) (string, string) {
			if v == "2" {
				return "ok", "Full address space randomization enabled"
			}
			if v == "1" {
				return "warn", "Partial address space randomization"
			}
			return "critical", "Address space randomization disabled"
		}),
		sysctlCheck("kptr_restrict", "Kernel pointer restriction", "kernel", "/proc/sys/kernel/kptr_restrict", func(v string) (string, string) {
			if atoi(v) >= 1 {
				return "ok", "Kernel pointers are restricted"
			}
			return "warn", "Kernel pointers may be exposed to userspace"
		}),
		sysctlCheck("dmesg_restrict", "Dmesg restriction", "kernel", "/proc/sys/kernel/dmesg_restrict", func(v string) (string, string) {
			if v == "1" {
				return "ok", "Unprivileged dmesg access is restricted"
			}
			return "warn", "Unprivileged users may read kernel logs"
		}),
		sysctlCheck("unpriv_bpf", "Unprivileged BPF", "kernel", "/proc/sys/kernel/unprivileged_bpf_disabled", func(v string) (string, string) {
			if atoi(v) >= 1 {
				return "ok", "Unprivileged BPF is disabled"
			}
			return "warn", "Unprivileged BPF is enabled"
		}),
		sysctlCheck("ipv4_forward", "IPv4 forwarding", "network", "/proc/sys/net/ipv4/ip_forward", func(v string) (string, string) {
			if v == "0" {
				return "ok", "IPv4 forwarding disabled"
			}
			return "warn", "IPv4 forwarding enabled"
		}),
		sysctlCheck("ipv6_forward", "IPv6 forwarding", "network", "/proc/sys/net/ipv6/conf/all/forwarding", func(v string) (string, string) {
			if v == "0" {
				return "ok", "IPv6 forwarding disabled"
			}
			return "warn", "IPv6 forwarding enabled"
		}),
		fileModeCheck("users_file", "Users file permissions", "auth", "settings/users.json", 0o077),
	}

	services := collectSecurityServices()
	processes := collectSecurityProcesses()
	listeners := collectSecurityListeners()
	checks = append(checks, failedServicesCheck())
	checks = append(checks, listenerExposureCheck(listeners))

	summary := SecuritySummary{}
	for _, c := range checks {
		addSecurityStatus(&summary, c.Status)
	}
	for _, s := range services {
		if s.Critical {
			addSecurityStatus(&summary, s.Status)
		}
	}

	return SecurityStatus{
		Summary:   summary,
		Checks:    checks,
		Services:  services,
		Processes: processes,
		Listeners: listeners,
	}
}

func sysctlCheck(key, label, category, path string, eval func(string) (string, string)) SecurityCheck {
	v := strings.TrimSpace(readFirstFile(path))
	if v == "" {
		return SecurityCheck{Key: key, Label: label, Category: category, Status: "unknown", Detail: "Not exposed by this kernel"}
	}
	status, detail := eval(v)
	return SecurityCheck{Key: key, Label: label, Category: category, Status: status, Detail: detail}
}

func fileModeCheck(key, label, category, path string, forbidden os.FileMode) SecurityCheck {
	st, err := os.Stat(path)
	if err != nil {
		return SecurityCheck{Key: key, Label: label, Category: category, Status: "unknown", Detail: "File not found from current working directory"}
	}
	mode := st.Mode().Perm()
	if mode&forbidden != 0 {
		return SecurityCheck{Key: key, Label: label, Category: category, Status: "warn", Detail: "File is readable or writable by group/other: " + mode.String()}
	}
	return SecurityCheck{Key: key, Label: label, Category: category, Status: "ok", Detail: "Restricted permissions: " + mode.String()}
}

func failedServicesCheck() SecurityCheck {
	failed := 0
	for _, svc := range CollectOpenRCServices() {
		if svc.State == ServiceCrashed {
			failed++
		}
	}
	if failed > 0 {
		return SecurityCheck{Key: "openrc_failed", Label: "OpenRC failed services", Category: "services", Status: "critical", Detail: strconv.Itoa(failed) + " failed service(s)"}
	}
	return SecurityCheck{Key: "openrc_failed", Label: "OpenRC failed services", Category: "services", Status: "ok", Detail: "No failed services reported by OpenRC"}
}

func collectSecurityServices() []SecurityService {
	watched := map[string]bool{
		"sshd": true, "dropbear": true, "nftables": true, "iptables": true,
		"ip6tables": true, "auditd": true, "chronyd": false, "syslog": false,
		"rsyslog": false, "crond": false, "acpid": false,
	}
	services := CollectOpenRCServices()
	byName := make(map[string]OpenRCService, len(services))
	for _, svc := range services {
		byName[svc.Name] = svc
	}

	result := make([]SecurityService, 0, len(watched))
	for name, critical := range watched {
		svc, ok := byName[name]
		if !ok {
			status := "unknown"
			if critical {
				status = "warn"
			}
			result = append(result, SecurityService{Name: name, State: "absent", Status: status, Critical: critical, Detail: "OpenRC service not installed"})
			continue
		}
		status := "ok"
		if svc.State == ServiceCrashed {
			status = "critical"
		} else if critical && svc.State != ServiceStarted {
			status = "warn"
		}
		result = append(result, SecurityService{
			Name:      name,
			State:     string(svc.State),
			Status:    status,
			Critical:  critical,
			Runlevels: svc.Runlevels,
			Detail:    "OpenRC state: " + string(svc.State),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func collectSecurityProcesses() []SecurityProcess {
	targets := []string{"unyport", "sshd", "dropbear", "xenstored", "xenconsoled", "qemu-system", "dnsmasq", "dockerd", "containerd"}
	found := scanProcesses(targets)
	result := make([]SecurityProcess, 0, len(targets))
	for _, name := range targets {
		pids := found[name]
		status := "unknown"
		detail := "Not running"
		if len(pids) > 0 {
			status = "ok"
			detail = strconv.Itoa(len(pids)) + " process(es)"
		}
		result = append(result, SecurityProcess{Name: name, Count: len(pids), Status: status, PIDs: pids, Detail: detail})
	}
	return result
}

func scanProcesses(targets []string) map[string][]int {
	result := make(map[string][]int, len(targets))
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return result
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		comm := strings.TrimSpace(readFirstFile("/proc/" + e.Name() + "/comm"))
		cmd := strings.TrimSpace(strings.ReplaceAll(readFirstFile("/proc/"+e.Name()+"/cmdline"), "\x00", " "))
		for _, target := range targets {
			if comm == target || strings.Contains(comm, target) || strings.Contains(cmd, target) {
				result[target] = append(result[target], pid)
			}
		}
	}
	for name := range result {
		sort.Ints(result[name])
	}
	return result
}

func collectSecurityListeners() []SecurityListener {
	listeners := append(parseTCPListeners("/proc/net/tcp", "tcp4"), parseTCPListeners("/proc/net/tcp6", "tcp6")...)
	sort.Slice(listeners, func(i, j int) bool {
		if listeners[i].Port != listeners[j].Port {
			return listeners[i].Port < listeners[j].Port
		}
		return listeners[i].Proto < listeners[j].Proto
	})
	return listeners
}

func parseTCPListeners(path, proto string) []SecurityListener {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	seen := map[int]bool{}
	var result []SecurityListener
	for i, line := range strings.Split(string(data), "\n") {
		if i == 0 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[3] != "0A" {
			continue
		}
		parts := strings.Split(fields[1], ":")
		if len(parts) != 2 {
			continue
		}
		port64, err := strconv.ParseInt(parts[1], 16, 32)
		if err != nil {
			continue
		}
		port := int(port64)
		if seen[port] {
			continue
		}
		seen[port] = true
		status := "ok"
		detail := "Listening TCP port"
		if port <= 1024 {
			status = "warn"
			detail = "Privileged TCP port exposed"
		}
		result = append(result, SecurityListener{Proto: proto, Port: port, Status: status, Detail: detail})
	}
	return result
}

func listenerExposureCheck(listeners []SecurityListener) SecurityCheck {
	privileged := 0
	for _, l := range listeners {
		if l.Port <= 1024 {
			privileged++
		}
	}
	if privileged > 0 {
		return SecurityCheck{Key: "listening_ports", Label: "Listening TCP ports", Category: "network", Status: "warn", Detail: strconv.Itoa(privileged) + " privileged TCP listener(s)"}
	}
	return SecurityCheck{Key: "listening_ports", Label: "Listening TCP ports", Category: "network", Status: "ok", Detail: strconv.Itoa(len(listeners)) + " TCP listener(s), none privileged"}
}

func addSecurityStatus(summary *SecuritySummary, status string) {
	switch status {
	case "ok":
		summary.OK++
	case "warn":
		summary.Warn++
	case "critical":
		summary.Critical++
	default:
		summary.Unknown++
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
