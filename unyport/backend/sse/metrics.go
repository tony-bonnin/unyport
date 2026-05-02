package sse

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Snapshot représente un relevé instantané des métriques système.
type Snapshot struct {
	Timestamp   time.Time `json:"ts"`
	CPUUsage    float64   `json:"cpu_usage"`
	CPUFreqAvg  int       `json:"cpu_freq_avg_mhz"`
	CPUFreqMax  int       `json:"cpu_freq_max_mhz"`
	MemTotal    uint64    `json:"mem_total_mb"`
	MemUsed     uint64    `json:"mem_used_mb"`
	MemFree     uint64    `json:"mem_free_mb"`
	NetRXBytes  uint64    `json:"net_rx_bytes"`
	NetTXBytes  uint64    `json:"net_tx_bytes"`
}

// collect lit /proc et retourne un Snapshot.
// Deux lectures CPU séparées de 200ms pour calculer l'usage.
func collect() Snapshot {
	t1 := readCPUTimes()
	time.Sleep(200 * time.Millisecond)
	t2 := readCPUTimes()

	cpuUsage := cpuPercent(t1, t2)
	freqAvg, freqMax := readFreqs()
	memTotal, memFree := readMem()
	rxBytes, txBytes := readNetBytes()

	return Snapshot{
		Timestamp:  time.Now(),
		CPUUsage:   cpuUsage,
		CPUFreqAvg: freqAvg,
		CPUFreqMax: freqMax,
		MemTotal:   memTotal / 1024,
		MemFree:    memFree / 1024,
		MemUsed:    (memTotal - memFree) / 1024,
		NetRXBytes: rxBytes,
		NetTXBytes: txBytes,
	}
}

// ---- CPU ----

type cpuTimes struct{ idle, total uint64 }

func readCPUTimes() map[string]cpuTimes {
	data, _ := os.ReadFile("/proc/stat")
	out := make(map[string]cpuTimes)
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu") || strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		vals := parseUint64Fields(fields[1:])
		idle := vals[3] + vals[4] // idle + iowait
		total := sum(vals[:8])
		out[fields[0]] = cpuTimes{idle: idle, total: total}
	}
	return out
}

func cpuPercent(t1, t2 map[string]cpuTimes) float64 {
	var total, count float64
	for cpu, a := range t1 {
		b, ok := t2[cpu]
		if !ok {
			continue
		}
		dIdle := float64(b.idle - a.idle)
		dTotal := float64(b.total - a.total)
		if dTotal > 0 {
			total += (1 - dIdle/dTotal) * 100
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return total / count
}

func readFreqs() (avg, max int) {
	entries, err := os.ReadDir("/sys/devices/system/cpu/")
	if err != nil {
		return
	}
	var freqs []int
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "cpu") || len(name) <= 3 {
			continue
		}
		base := "/sys/devices/system/cpu/" + name + "/cpufreq/"
		if cur := readIntFile(base + "scaling_cur_freq"); cur > 0 {
			freqs = append(freqs, cur/1000)
		}
		if m := readIntFile(base + "cpuinfo_max_freq"); m/1000 > max {
			max = m / 1000
		}
	}
	if len(freqs) > 0 {
		s := 0
		for _, f := range freqs {
			s += f
		}
		avg = s / len(freqs)
	}
	return
}

// ---- Mémoire ----

func readMem() (total, available uint64) {
	data, _ := os.ReadFile("/proc/meminfo")
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			total = parseKB(line)
		case strings.HasPrefix(line, "MemAvailable:"):
			available = parseKB(line)
		}
	}
	return
}

// ---- Réseau ----

func readNetBytes() (rx, tx uint64) {
	data, _ := os.ReadFile("/proc/net/dev")
	for _, line := range strings.Split(string(data), "\n")[2:] {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		iface := strings.Trim(fields[0], ":")
		if iface == "lo" {
			continue
		}
		rx += parseU64(fields[1])
		tx += parseU64(fields[9])
	}
	return
}

// ---- Helpers ----

func readIntFile(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	v, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return v
}

func parseKB(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	v, _ := strconv.ParseUint(fields[1], 10, 64)
	return v
}

func parseU64(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}

func parseUint64Fields(fields []string) []uint64 {
	out := make([]uint64, len(fields))
	for i, f := range fields {
		out[i], _ = strconv.ParseUint(f, 10, 64)
	}
	return out
}

func sum(v []uint64) uint64 {
	var s uint64
	for _, x := range v {
		s += x
	}
	return s
}

// readFirstFile lit le premier fichier accessible parmi les chemins donnés.
func readFirstFile(paths ...string) string {
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

func getCPUInfo() (model, vendor string, cores int) {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "unknown", "unknown", 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "model name") && model == "" {
			if p := strings.SplitN(line, ":", 2); len(p) > 1 {
				model = strings.TrimSpace(p[1])
			}
		}
		if strings.HasPrefix(line, "vendor_id") && vendor == "" {
			if p := strings.SplitN(line, ":", 2); len(p) > 1 {
				vendor = strings.TrimSpace(p[1])
			}
		}
		if strings.HasPrefix(line, "processor") {
			cores++
		}
	}
	return
}

func getOSRelease() (pretty, version string) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "unknown", "unknown"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			pretty = strings.Trim(line[len("PRETTY_NAME="):], `"`)
		}
		if strings.HasPrefix(line, "VERSION_ID=") {
			version = strings.Trim(line[len("VERSION_ID="):], `"`)
		}
	}
	if pretty == "" { pretty = "unknown" }
	if version == "" { version = "unknown" }
	return
}