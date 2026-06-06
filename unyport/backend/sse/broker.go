package sse

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	pollInterval = 2 * time.Second
	ringSize     = 60 // 2 minutes d'historique en mémoire
)

// Broker collecte les métriques et diffuse vers N clients SSE connectés.
type Broker struct {
	mu            sync.RWMutex
	ring          [ringSize]Snapshot
	head          int
	count         int
	clients       map[chan Snapshot]struct{}
	logger        *slog.Logger
	hostRole      HostRole // détecté une fois au démarrage, immuable
	rebootLogPath string
	startupHistoryPath string
}

func NewBroker(logger *slog.Logger, rebootLogPath string, startupHistoryPath string) *Broker {
	role := DetectHostRole()
	logger.Info("host role detected",
		"role", role.Role,
		"runtime", role.Runtime,
		"label", role.Label,
		"verified", role.Verified,
	)

	b := &Broker{
		clients:  make(map[chan Snapshot]struct{}),
		logger:   logger,
		hostRole: role,
		rebootLogPath: rebootLogPath,
		startupHistoryPath: startupHistoryPath,
	}
	go b.loop()
	return b
}

func (b *Broker) loop() {
	var prevSnap Snapshot
	var prevIfaces map[string]NetIfaceInfo
	var prevXenCPU map[int]float64
	for {
		snap := collect()

		// Calcul débit réseau (bytes/s) entre deux snapshots
		if prevSnap.Timestamp.IsZero() {
			snap.NetRXBps = 0
			snap.NetTXBps = 0
		} else {
			dt := snap.Timestamp.Sub(prevSnap.Timestamp).Seconds()
			if dt > 0 {
				if snap.NetRXBytes >= prevSnap.NetRXBytes {
					snap.NetRXBps = uint64(float64(snap.NetRXBytes-prevSnap.NetRXBytes) / dt)
				}
				if snap.NetTXBytes >= prevSnap.NetTXBytes {
					snap.NetTXBps = uint64(float64(snap.NetTXBytes-prevSnap.NetTXBytes) / dt)
				}
			}
		}

		// Network map — collecte avec débits par interface
		dt := 0.0
		if !prevSnap.Timestamp.IsZero() {
			dt = snap.Timestamp.Sub(prevSnap.Timestamp).Seconds()
		}
		netMap, currentIfaces := collectNetworkMap(prevIfaces, dt)
		snap.NetMap = netMap
		prevIfaces = currentIfaces

		// Xen Dom0 — utiliser la toolstack Xen pour les domaines/hyperviseur.
		// Les métriques Linux /proc restent présentes, mais ces champs donnent
		// la vue correcte de l'hyperviseur au lieu du seul noyau Alpine Dom0.
		if b.hostRole.Role == "Dom0" {
			var xenInfo XenInfo
			snap.XenDomains, xenInfo, prevXenCPU = collectXenSnapshot(prevXenCPU, dt)
			snap.XenInfo = xenInfo
		}

		prevSnap = snap

		b.mu.Lock()

		// ── Échelles oscilloscope ─────────────────────────────────────────
		// Calcul sur la fenêtre visible (15 snapshots = ~30s à 2s/tick).
		// Go lit le ring courant et calcule min/max réels des données —
		// le frontend applique sans calcul (calibrage automatique).
		const windowSnaps = 15
		snap = b.computeOscilloScales(snap, windowSnaps)

		b.ring[b.head] = snap
		b.head = (b.head + 1) % ringSize
		if b.count < ringSize {
			b.count++
		}
		for ch := range b.clients {
			select {
			case ch <- snap:
			default:
				// client trop lent, on drop
			}
		}
		b.mu.Unlock()
		time.Sleep(pollInterval)
	}
}

func (b *Broker) subscribe() chan Snapshot {
	ch := make(chan Snapshot, 4)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Broker) unsubscribe(ch chan Snapshot) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
}

// Handler est le endpoint SSE — protégé par AuthMiddleware en amont.
func (b *Broker) Handler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE non supporté", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := b.subscribe()
	defer b.unsubscribe(ch)

	b.logger.Debug("sse client connected", "remote", r.RemoteAddr)

	// Envoie le dernier snapshot immédiatement si dispo
	b.mu.RLock()
	if b.count > 0 {
		last := b.ring[(b.head-1+ringSize)%ringSize]
		b.mu.RUnlock()
		writeSSE(w, flusher, last)
	} else {
		b.mu.RUnlock()
	}

	for {
		select {
		case snap := <-ch:
			writeSSE(w, flusher, snap)
		case <-r.Context().Done():
			b.logger.Debug("sse client disconnected", "remote", r.RemoteAddr)
			return
		}
	}
}

// computeOscilloScales calcule les échelles "oscilloscope" pour le Snapshot courant.
// Lit les N derniers snapshots du ring (fenêtre visible), calcule min/max réels,
// ajoute une marge de 10% et arrondit proprement.
// Le frontend se contente d'appliquer cpu_y_min/max, cpu_freq_y_min/max, mem_y_min/max.
func (b *Broker) computeOscilloScales(snap Snapshot, window int) Snapshot {
	// Collecter les valeurs sur la fenêtre (ring déjà verrouillé par l'appelant)
	n := b.count
	if n > window {
		n = window
	}

	// CPU usage
	cpuMin, cpuMax := snap.CPUUsage, snap.CPUUsage
	// Fréquence
	freqMin, freqMax := snap.CPUFreqAvg, snap.CPUFreqAvg
	// Mémoire
	memMin, memMax := snap.MemUsed, snap.MemUsed

	for i := 1; i < n; i++ {
		idx := (b.head - 1 - i + ringSize) % ringSize
		s := b.ring[idx]
		if s.Timestamp.IsZero() {
			continue
		}
		if s.CPUUsage < cpuMin {
			cpuMin = s.CPUUsage
		}
		if s.CPUUsage > cpuMax {
			cpuMax = s.CPUUsage
		}
		if s.CPUFreqAvg > 0 {
			if s.CPUFreqAvg < freqMin {
				freqMin = s.CPUFreqAvg
			}
			if s.CPUFreqAvg > freqMax {
				freqMax = s.CPUFreqAvg
			}
		}
		if s.MemUsed < memMin {
			memMin = s.MemUsed
		}
		if s.MemUsed > memMax {
			memMax = s.MemUsed
		}
	}

	// ── CPU : échelle = span ± 5% du span ─────────────────────────────
	cpuSpan := cpuMax - cpuMin
	if cpuSpan < 0.5 {
		cpuSpan = 0.5
	}
	cpuMargin := cpuSpan * 0.05
	if cpuMargin < 0.1 {
		cpuMargin = 0.1
	}
	snap.CPUYMin = cpuMin - cpuMargin
	if snap.CPUYMin < 0 {
		snap.CPUYMin = 0
	}
	snap.CPUYMax = cpuMax + cpuMargin
	if snap.CPUYMax > 100.0 {
		snap.CPUYMax = 100.0
	}

	// ── Fréquence : échelle = span ± 5% du span ───────────────────────
	// span = max - min sur la fenêtre (ring buffer).
	// Si span = 1 MHz → marge = 0.05 MHz de chaque côté.
	// Span minimum : 1 MHz pour qu'il y ait toujours une fenêtre visible.
	freqSpan := freqMax - freqMin
	if freqSpan < 1 {
		freqSpan = 1
	}
	freqMargin := freqSpan / 20 // 5% du span
	if freqMargin < 1 {
		freqMargin = 1
	}
	snap.CPUFreqYMin = freqMin - freqMargin
	if snap.CPUFreqYMin < 0 {
		snap.CPUFreqYMin = 0
	}
	snap.CPUFreqYMax = freqMax + freqMargin

	// ── Mémoire : échelle = span ± 5% du span ─────────────────────────
	memSpan := memMax - memMin
	if memSpan < 1 {
		memSpan = 1
	}
	memMargin := memSpan / 20 // 5% du span
	if memMargin < 1 {
		memMargin = 1
	}
	if memMin > memMargin {
		snap.MemYMin = memMin - memMargin
	} else {
		snap.MemYMin = 0
	}
	snap.MemYMax = memMax + memMargin
	if snap.MemYMax > snap.MemTotal {
		snap.MemYMax = snap.MemTotal
	}

	return snap
}

func mathMax(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func mathMin(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func writeSSE(w http.ResponseWriter, f http.Flusher, snap Snapshot) {
	data, err := json.Marshal(snap)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	f.Flush()
}

// ============================================================
// SystemInfo — réponse de /api/system
// ============================================================

// SystemInfo contient les infos statiques HW/OS + le rôle de l'hôte.
// Les champs mem_*_mb sont cohérents avec les clés du Snapshot SSE.
type SystemInfo struct {
	// OS / Kernel
	Hostname  string `json:"hostname"`
	OSRelease string `json:"os_release"`
	OSVersion string `json:"os_version"`
	Kernel    string `json:"kernel"`
	Date      string `json:"date"`
	Uptime    string `json:"uptime"`

	// CPU
	CPUModel  string `json:"cpu_model"`
	CPUVendor string `json:"cpu_vendor"`
	CPUCores  int    `json:"cpu_cores"`

	// Carte mère (absent en container/DomU)
	BoardName    string `json:"board_name"`
	BoardVendor  string `json:"board_vendor"`
	BoardVersion string `json:"board_version"` // portage health-model.lua

	// BIOS / Firmware (absent en VM paravirt et container)
	BIOS BIOSInfo `json:"bios"`

	// GPUs détectés (absent en DomU paravirt headless)
	GPUs []GPUInfo `json:"gpus"`

	// APK summary (count paquets installés)
	APK APKSummary `json:"apk"`

	// Kernel modules summary (count)
	Modules ModulesSummary `json:"modules"`

	// Rôle hôte — clé unique pour toute la logique UI
	HostRole HostRole `json:"host_role"`

	// Réseau
	NetIface string `json:"net_iface"`
	NetIP    string `json:"net_ip"`

	// Métriques live (snapshot le plus récent au moment de l'appel)
	CPUUsage       float64   `json:"cpu_usage"`
	CPUPerCore     []float64 `json:"cpu_per_core"`
	CPUFreqAvg     int       `json:"cpu_freq_avg_mhz"`
	CPUFreqMax     int       `json:"cpu_freq_max_mhz"`
	CPUFreqCore    []int     `json:"cpu_freq_core"`
	CPUYMin        float64   `json:"cpu_y_min"`
	CPUYMax        float64   `json:"cpu_y_max"`
	CPUFreqYMin    int       `json:"cpu_freq_y_min"`
	CPUFreqYMax    int       `json:"cpu_freq_y_max"`
	MemYMin        uint64    `json:"mem_y_min"`
	MemYMax        uint64    `json:"mem_y_max"`
	CPUPeakCores   []int     `json:"cpu_peak_cores"`
	MemTotalMB     uint64    `json:"mem_total_mb"`
	MemUsedMB      uint64    `json:"mem_used_mb"`
	MemCachedMB    uint64    `json:"mem_cached_mb"`
	MemFreeMB      uint64    `json:"mem_free_mb"`
	MemStackUsed   uint64    `json:"mem_stack_used"`
	MemStackCached uint64    `json:"mem_stack_cached"`
	MemStackFree   uint64    `json:"mem_stack_free"`
	MemUsedPct     uint8     `json:"mem_used_pct"`
	MemCachedPct   uint8     `json:"mem_cached_pct"`

	// Compat legacy — certains appels utilisaient xen_role directement
	XenRole string `json:"xen_role"`

	// LBU — Alpine Linux Backup (absent si LBU non installé)
	LBU LBUStatus `json:"lbu"`

	// Xen Dom0 — hyperviseur et domaines vus via xl.
	XenInfo    XenInfo     `json:"xen_info"`
	XenDomains []XenDomain `json:"xen_domains"`
}

// SystemInfoHandler retourne un snapshot complet (statique + live) en JSON.
func (b *Broker) SystemInfoHandler(w http.ResponseWriter, r *http.Request) {
	b.logger.Debug("system info requested",
		"host_role", b.hostRole.Role,
		"runtime", b.hostRole.Runtime,
		"verified", b.hostRole.Verified,
	)
	model, vendor, cores := getCPUInfo()
	pretty, version := getOSRelease()
	kernel := readFirstFile("/proc/sys/kernel/osrelease")
	hostname := readFirstFile("/proc/sys/kernel/hostname")
	uptime := readFirstFile("/proc/uptime")
	board := readFirstFile("/sys/devices/virtual/dmi/id/board_name")
	bvendor := readFirstFile("/sys/devices/virtual/dmi/id/board_vendor")

	// Nouveaux champs — portage ACF Lua
	boardVersion := CollectBoardVersion()
	bios := CollectBIOSInfo()
	gpus := CollectGPUs()
	apkSummary := CollectAPKSummary()
	modSummary := CollectModulesSummary()

	b.mu.RLock()
	var snap Snapshot
	if b.count > 0 {
		snap = b.ring[(b.head-1+ringSize)%ringSize]
	}
	b.mu.RUnlock()

	// Compat legacy : xen_role déduit de HostRole
	xenRole := ""
	switch b.hostRole.Role {
	case "Dom0", "DomU":
		xenRole = b.hostRole.Role
	}

	info := SystemInfo{
		Hostname:       strings.TrimSpace(hostname),
		OSRelease:      pretty,
		OSVersion:      version,
		Kernel:         strings.TrimSpace(kernel),
		Date:           time.Now().Format(time.RFC1123),
		Uptime:         strings.TrimSpace(uptime),
		CPUModel:       model,
		CPUVendor:      vendor,
		CPUCores:       cores,
		BoardName:      strings.TrimSpace(board),
		BoardVendor:    strings.TrimSpace(bvendor),
		BoardVersion:   boardVersion,
		BIOS:           bios,
		GPUs:           gpus,
		APK:            apkSummary,
		Modules:        modSummary,
		HostRole:       b.hostRole,
		XenRole:        xenRole,
		NetIface:       snap.NetIface,
		NetIP:          snap.NetIP,
		CPUUsage:       snap.CPUUsage,
		CPUPerCore:     snap.CPUPerCore,
		CPUFreqAvg:     snap.CPUFreqAvg,
		CPUFreqMax:     snap.CPUFreqMax,
		CPUFreqCore:    snap.CPUFreqCore,
		CPUYMin:        snap.CPUYMin,
		CPUYMax:        snap.CPUYMax,
		CPUFreqYMin:    snap.CPUFreqYMin,
		CPUFreqYMax:    snap.CPUFreqYMax,
		MemYMin:        snap.MemYMin,
		MemYMax:        snap.MemYMax,
		CPUPeakCores:   snap.CPUPeakCores,
		MemTotalMB:     snap.MemTotal,
		MemUsedMB:      snap.MemUsed,
		MemCachedMB:    snap.MemCached,
		MemFreeMB:      snap.MemFree,
		MemStackUsed:   snap.MemStackUsed,
		MemStackCached: snap.MemStackCached,
		MemStackFree:   snap.MemStackFree,
		MemUsedPct:     snap.MemUsedPct,
		MemCachedPct:   snap.MemCachedPct,
		LBU:            snap.LBU,
		XenInfo:        snap.XenInfo,
		XenDomains:     snap.XenDomains,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(info)
}

// VersionsHandler — GET /api/versions
// Fetch la page publique https://github.com/trinity-labs/trinity-boot/releases
// côté serveur (pas de CSP côté client), parse les tags via regex,
// retourne les versions latest kernel + alpine filtrées par rôle hôte.
//
// Réponse JSON :
//
//	{ "kernel_lts": "6.18.33-lts", "alpine": "3.23.4", "role": "dom0" }
func (b *Broker) VersionsHandler(w http.ResponseWriter, r *http.Request) {
	// Rôle hôte → slug pour filtrer les tags TRINITY
	roleSlug := "dom0"
	switch b.hostRole.Role {
	case "DomU":
		roleSlug = "domU"
	}

	// Fetch flux Atom public — plus stable que le HTML rendu des releases.
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get("https://github.com/trinity-labs/trinity-boot/releases.atom")
	if err != nil {
		http.Error(w, `{"error":"fetch_failed"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024)) // max 512 KB
	if err != nil {
		http.Error(w, `{"error":"read_failed"}`, http.StatusBadGateway)
		return
	}
	feed := string(body)

	// Regex kernel : kernel-{role}-X.X.X-N-lts
	kerPat := regexp.MustCompile(`kernel-` + regexp.QuoteMeta(roleSlug) + `-(\d+\.\d+\.\d+)-\d+-lts`)
	// Regex alpine : alpine-{role}-X.X.X
	alpPat := regexp.MustCompile(`alpine-` + regexp.QuoteMeta(roleSlug) + `-(\d+\.\d+\.\d+)`)

	kernelVer := highestMatchedVersion(feed, kerPat)
	if kernelVer != "" {
		kernelVer += "-lts"
	}
	alpineVer := highestMatchedVersion(feed, alpPat)

	type versionsResp struct {
		KernelLts string `json:"kernel_lts"`
		Alpine    string `json:"alpine"`
		Role      string `json:"role"`
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// Cache 1h — les releases ne changent pas toutes les minutes
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_ = json.NewEncoder(w).Encode(versionsResp{
		KernelLts: strings.TrimSpace(kernelVer),
		Alpine:    strings.TrimSpace(alpineVer),
		Role:      roleSlug,
	})
}

func highestMatchedVersion(input string, pattern *regexp.Regexp) string {
	matches := pattern.FindAllStringSubmatch(input, -1)
	best := ""
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		if compareDotVersions(match[1], best) > 0 {
			best = match[1]
		}
	}
	return best
}

func compareDotVersions(a, b string) int {
	if a == b {
		return 0
	}
	if b == "" {
		return 1
	}

	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	maxLen := len(pa)
	if len(pb) > maxLen {
		maxLen = len(pb)
	}

	for i := 0; i < maxLen; i++ {
		av := 0
		bv := 0
		if i < len(pa) {
			fmt.Sscanf(pa[i], "%d", &av)
		}
		if i < len(pb) {
			fmt.Sscanf(pb[i], "%d", &bv)
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

type rebootHeatmapCell struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
	Level int    `json:"level"`
	Label string `json:"label"`
}

type rebootHeatmapResp struct {
	Year        int                 `json:"year"`
	TotalReboots int                `json:"total_reboots"`
	MaxPerDay   int                 `json:"max_per_day"`
	LeadBlank   int                 `json:"lead_blank"`
	Days        []rebootHeatmapCell `json:"days"`
}

type startupHistoryRecord struct {
	Timestamp  string `json:"timestamp"`
	Event      string `json:"event"`
	Theme      string `json:"theme,omitempty"`
	BootID     string `json:"boot_id,omitempty"`
	RebootedAt string `json:"rebooted_at,omitempty"`
}

var startupLogPattern = regexp.MustCompile(`time=([0-9T:\-:\.]+Z).*msg="?startup"?`)

func (b *Broker) RebootsHandler(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	year := now.Year()
	loc := now.Location()

	counts, err := parseStartupHistoryByDay(b.startupHistoryPath, year, loc)
	if err != nil || len(counts) == 0 {
		if err != nil {
			b.logger.Warn("startup history unavailable", "path", b.startupHistoryPath, "err", err)
		}
		counts, err = parseStartupRebootsByDay(b.rebootLogPath, year, loc)
		if err != nil {
			b.logger.Warn("startup log fallback unavailable", "path", b.rebootLogPath, "err", err)
		}
	}

	startOfYear := time.Date(year, time.January, 1, 0, 0, 0, 0, loc)
	daysInYear := 365
	if isLeapYear(year) {
		daysInYear = 366
	}

	cells := make([]rebootHeatmapCell, 0, daysInYear)
	totalReboots := 0
	maxPerDay := 0

	for i := 0; i < daysInYear; i++ {
		day := startOfYear.AddDate(0, 0, i)
		key := day.Format("2006-01-02")
		count := counts[key]
		level := rebootHeatmapLevel(count)
		if count > maxPerDay {
			maxPerDay = count
		}
		totalReboots += count
		cells = append(cells, rebootHeatmapCell{
			Date:  key,
			Count: count,
			Level: level,
			Label: day.Format("Mon 02 Jan 2006"),
		})
	}

	leadBlank := (int(startOfYear.Weekday()) + 6) % 7

	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(rebootHeatmapResp{
		Year:         year,
		TotalReboots: totalReboots,
		MaxPerDay:    maxPerDay,
		LeadBlank:    leadBlank,
		Days:         cells,
	})
}


func parseStartupRebootsByDay(path string, year int, loc *time.Location) (map[string]int, error) {
	out := make(map[string]int)
	file, err := os.Open(path)
	if err != nil {
		return out, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		match := startupLogPattern.FindStringSubmatch(line)
		if len(match) < 2 {
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, match[1])
		if err != nil {
			continue
		}
		ts = ts.In(loc)
		if ts.Year() != year {
			continue
		}
		out[ts.Format("2006-01-02")]++
	}
	return out, scanner.Err()
}

func parseStartupHistoryByDay(path string, year int, loc *time.Location) (map[string]int, error) {
	out := make(map[string]int)
	seenBootIDs := make(map[string]struct{})
	seenRebootTimes := make(map[string]struct{})
	file, err := os.Open(path)
	if err != nil {
		return out, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec startupHistoryRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec.Event != "startup" || (rec.Timestamp == "" && rec.RebootedAt == "") {
			continue
		}
		referenceTime := rec.RebootedAt
		if referenceTime == "" {
			referenceTime = rec.Timestamp
		}
		if rec.BootID != "" {
			if _, exists := seenBootIDs[rec.BootID]; exists {
				continue
			}
			seenBootIDs[rec.BootID] = struct{}{}
		} else if rec.RebootedAt != "" {
			if _, exists := seenRebootTimes[rec.RebootedAt]; exists {
				continue
			}
			seenRebootTimes[rec.RebootedAt] = struct{}{}
		}
		ts, err := time.Parse(time.RFC3339Nano, referenceTime)
		if err != nil {
			continue
		}
		ts = ts.In(loc)
		if ts.Year() != year {
			continue
		}
		out[ts.Format("2006-01-02")]++
	}
	return out, scanner.Err()
}

func rebootHeatmapLevel(count int) int {
	switch {
	case count <= 0:
		return 0
	case count == 1:
		return 1
	case count == 2:
		return 2
	default:
		return 3
	}
}

func isLeapYear(year int) bool {
	if year%400 == 0 {
		return true
	}
	if year%100 == 0 {
		return false
	}
	return year%4 == 0
}
