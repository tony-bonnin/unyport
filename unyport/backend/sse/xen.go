package sse

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const xenCommandTimeout = 900 * time.Millisecond

// XenDomain — un domaine Xen vu depuis le Dom0 (`xl list`, CPU% façon xentop).
type XenDomain struct {
	DomID  int     `json:"domid"`
	Name   string  `json:"name"`
	State  string  `json:"state"` // r/b/p/s/c/d (running/blocked/paused/shutdown/crashed/dying)
	VCPUs  int     `json:"vcpus"`
	MemMB  uint64  `json:"mem_mb"`
	CPUSec float64 `json:"cpu_sec"` // temps CPU cumulé (s) — base du calcul de %
	CPUPct float64 `json:"cpu_pct"` // % CPU — calculé par delta dans le broker
}

// XenInfo résume l'état de l'hyperviseur Xen vu depuis Dom0.
// Il complète les métriques Linux classiques, qui ne voient que le noyau Dom0.
type XenInfo struct {
	Available     bool   `json:"available"`
	Toolstack     string `json:"toolstack"`
	XenVersion    string `json:"xen_version"`
	Host          string `json:"host"`
	Scheduler     string `json:"scheduler"`
	CPUs          int    `json:"cpus"`
	TotalMemoryMB uint64 `json:"total_memory_mb"`
	FreeMemoryMB  uint64 `json:"free_memory_mb"`
	DomainCount   int    `json:"domain_count"`
	Running       int    `json:"running"`
	Blocked       int    `json:"blocked"`
	Paused        int    `json:"paused"`
	Shutdown      int    `json:"shutdown"`
	Crashed       int    `json:"crashed"`
	Dying         int    `json:"dying"`
	TotalVCPUs    int    `json:"total_vcpus"`
	DomainMemMB   uint64 `json:"domain_mem_mb"`
}

// collectXenSnapshot lit `xl list` + `xl info` et renvoie une vision Dom0.
// Le CPU% est calculé par delta de CPU seconds, comme xentop/xl top, mais sans
// bloquer la boucle SSE avec un sampler externe.
//
// IMPORTANT : à n'appeler QUE si HostRole == "Dom0". Sur DomU/Container/baremetal
// la commande `xl` est soit absente, soit sans privilège toolstack → renvoie nil.
func collectXenSnapshot(prev map[int]float64, dt float64) ([]XenDomain, XenInfo, map[int]float64) {
	doms := collectXenDomains()
	cur := computeXenCPUPct(doms, prev, dt)
	info := collectXenInfo()
	info.Available = info.Available || len(doms) > 0
	info.Toolstack = "xl"
	info.DomainCount = len(doms)

	for _, d := range doms {
		info.TotalVCPUs += d.VCPUs
		info.DomainMemMB += d.MemMB
		for _, c := range d.State {
			switch c {
			case 'r':
				info.Running++
			case 'b':
				info.Blocked++
			case 'p':
				info.Paused++
			case 's':
				info.Shutdown++
			case 'c':
				info.Crashed++
			case 'd':
				info.Dying++
			}
		}
	}

	return doms, info, cur
}

// collectXenDomains lit `xl list` (instantané) et renvoie les domaines.
// `xl list` expose le même compteur CPU cumulé que xentop/xl top ; le broker
// transforme ensuite ce compteur en pourcentage par delta entre deux ticks.
//
// Format attendu (`xl list`) :
//
//	Name                  ID   Mem VCPUs      State   Time(s)
//	Domain-0               0  2048     4     r-----    1234.5
func collectXenDomains() []XenDomain {
	out, err := runXLCmd("list")
	if err != nil {
		return nil
	}
	lines := strings.Split(string(out), "\n")
	doms := make([]XenDomain, 0, len(lines))
	for i, line := range lines {
		if i == 0 { // en-tête
			continue
		}
		f := strings.Fields(line)
		if len(f) < 6 {
			continue
		}
		domid, err := strconv.Atoi(f[1])
		if err != nil {
			continue
		}
		mem, _ := strconv.ParseUint(f[2], 10, 64)
		vcpus, _ := strconv.Atoi(f[3])
		cpuSec, _ := strconv.ParseFloat(f[len(f)-1], 64)
		doms = append(doms, XenDomain{
			DomID:  domid,
			Name:   f[0],
			MemMB:  mem,
			VCPUs:  vcpus,
			State:  strings.Trim(f[4], "-"),
			CPUSec: cpuSec,
		})
	}
	return doms
}

func collectXenInfo() XenInfo {
	out, err := runXLCmd("info")
	if err != nil {
		return XenInfo{}
	}
	info := XenInfo{Available: true, Toolstack: "xl"}
	for _, line := range strings.Split(string(out), "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "host":
			info.Host = val
		case "xen_version":
			info.XenVersion = val
		case "scheduler":
			info.Scheduler = val
		case "nr_cpus":
			info.CPUs, _ = strconv.Atoi(val)
		case "total_memory":
			info.TotalMemoryMB = parseXenMemoryMB(val)
		case "free_memory":
			info.FreeMemoryMB = parseXenMemoryMB(val)
		}
	}
	return info
}

func runXLCmd(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), xenCommandTimeout)
	defer cancel()
	return exec.CommandContext(ctx, "xl", args...).Output()
}

func parseXenMemoryMB(s string) uint64 {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return 0
	}
	v, _ := strconv.ParseUint(fields[0], 10, 64)
	return v
}

// computeXenCPUPct calcule le %CPU de chaque domaine par delta de temps CPU
// cumulé entre deux relevés. prev = CPUSec du tick précédent indexé par DomID.
// dt = secondes écoulées. Retourne la map des CPUSec courants (à conserver pour
// le prochain tick). Le % est borné à [0, 100 × vcpus].
func computeXenCPUPct(doms []XenDomain, prev map[int]float64, dt float64) map[int]float64 {
	cur := make(map[int]float64, len(doms))
	for i := range doms {
		d := &doms[i]
		cur[d.DomID] = d.CPUSec
		if dt <= 0 {
			continue
		}
		if p, ok := prev[d.DomID]; ok && d.CPUSec >= p {
			pct := (d.CPUSec - p) / dt * 100.0
			if max := float64(d.VCPUs) * 100.0; d.VCPUs > 0 && pct > max {
				pct = max
			}
			d.CPUPct = pct
		}
	}
	return cur
}
