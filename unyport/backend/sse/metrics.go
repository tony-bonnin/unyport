package sse

import (
	"math"
	"net"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// HostRole décrit le rôle de l'hôte dans l'écosystème TRINITY.
// Valeurs possibles : "Dom0", "DomU", "Container", "Alpine", "Unknown".
type HostRole struct {
	Role     string `json:"role"`     // Dom0 | DomU | Container | Alpine | Unknown
	Runtime  string `json:"runtime"`  // xen | podman | docker | lxc | containerd | native
	Label    string `json:"label"`    // Label lisible — affiché dans l'UI
	Verified bool   `json:"verified"` // true = détection par preuve directe (pas heuristique)
}

// Snapshot représente un relevé instantané des métriques système.
// Toutes les valeurs dérivées sont calculées ici en Go —
// le frontend se contente de les afficher.
type Snapshot struct {
	Timestamp time.Time `json:"ts"`

	// ── CPU ──────────────────────────────────────────────────────
	CPUUsage    float64   `json:"cpu_usage"`        // % moyen global
	CPUPerCore  []float64 `json:"cpu_per_core"`     // % par core, trié
	CPUFreqAvg  int       `json:"cpu_freq_avg_mhz"` // fréquence moyenne
	CPUFreqMax  int       `json:"cpu_freq_max_mhz"` // fréquence max théorique
	CPUFreqCore []int     `json:"cpu_freq_core"`    // fréquence courante par core
	// Échelles oscilloscope — calculées sur la fenêtre visible (ring buffer)
	CPUYMin     float64 `json:"cpu_y_min"`      // plancher axe CPU (données réelles -10%)
	CPUYMax     float64 `json:"cpu_y_max"`      // plafond axe CPU (données réelles +10%)
	CPUFreqYMin int     `json:"cpu_freq_y_min"` // plancher axe fréquence
	CPUFreqYMax int     `json:"cpu_freq_y_max"` // plafond axe fréquence
	MemYMin     uint64  `json:"mem_y_min"`      // plancher axe mémoire (MiB)
	MemYMax     uint64  `json:"mem_y_max"`      // plafond axe mémoire (MiB)
	// Cores dont l'écart vs moyenne dépasse 20% → pic visible
	CPUPeakCores []int `json:"cpu_peak_cores"`

	// ── Mémoire ──────────────────────────────────────────────────
	MemTotal  uint64 `json:"mem_total_mb"`  // RAM totale
	MemUsed   uint64 `json:"mem_used_mb"`   // utilisée (hors cache)
	MemCached uint64 `json:"mem_cached_mb"` // cache + buffers
	MemFree   uint64 `json:"mem_free_mb"`   // libre
	// Valeurs cumulées pour empilement Chart.js (fill:origin) — calculées en Go
	MemStackUsed   uint64 `json:"mem_stack_used"`   // = mem_used_mb
	MemStackCached uint64 `json:"mem_stack_cached"` // = used + cached
	MemStackFree   uint64 `json:"mem_stack_free"`   // = total
	// Pourcentages pré-calculés (0-100)
	MemUsedPct   uint8 `json:"mem_used_pct"`
	MemCachedPct uint8 `json:"mem_cached_pct"`

	// ── Réseau ───────────────────────────────────────────────────
	NetRXBytes uint64 `json:"net_rx_bytes"`
	NetTXBytes uint64 `json:"net_tx_bytes"`
	NetRXBps   uint64 `json:"net_rx_bps"`
	NetTXBps   uint64 `json:"net_tx_bps"`
	NetIface   string `json:"net_iface"`
	NetIP      string `json:"net_ip"`

	// ── LBU (Alpine Linux Backup) ─────────────────────────────
	LBU LBUStatus `json:"lbu"`

	// ── Network Map ───────────────────────────────────────────
	NetMap NetworkMap `json:"net_map"`

	// ── Disques ───────────────────────────────────────────────
	Disks []DiskMount `json:"disks"`

	// ── Load average ──────────────────────────────────────────
	LoadAvg LoadAverage `json:"load_avg"`

	// ── Températures CPU ──────────────────────────────────────
	CPUTemps []CPUTemp `json:"cpu_temps"`

	// ── Top processus ─────────────────────────────────────────
	TopProcs []ProcInfo `json:"top_procs"`

	// ── Xen Dom0 ──────────────────────────────────────────────
	// Renseigné uniquement quand le rôle détecté est Dom0 et que xl répond.
	XenInfo    XenInfo     `json:"xen_info"`
	XenDomains []XenDomain `json:"xen_domains"`
}

// LBUStatus représente l'état de la persistance LBU.
//
// State :
//
//	"absent"  → /etc/apk/protected-paths.d inexistant : LBU non installé,
//	            la card LBU ne doit PAS être affichée dans l'UI.
//	"clean"   → archive LBU présente et plus récente que protected-paths.d :
//	            la configuration est persistée.
//	"dirty"   → protected-paths.d plus récent que l'archive LBU (ou archive absente) :
//	            des changements ne sont pas encore commités.
//
// Détection purement par stat(2) — zéro binaire externe, zéro exec.
// Référence : /etc/apk/protected-paths.d (présence = LBU installé)
//
//	/var/cache/lbu/          (contient *.tar.gz horodatés)
type LBUStatus struct {
	Present bool   `json:"present"` // false → LBU absent, card masquée
	State   string `json:"state"`   // "absent" | "clean" | "dirty"
	Archive string `json:"archive"` // basename de l'archive active, "" si absente
}

// collect lit /proc et retourne un Snapshot avec toutes les valeurs
// pré-calculées pour le frontend. Go fait les maths, JS affiche.
func collect() Snapshot {
	t1 := readCPUTimes()
	time.Sleep(200 * time.Millisecond)
	t2 := readCPUTimes()

	cpuUsage, perCore := cpuPercentAll(t1, t2)
	_, freqMax, freqCore := readFreqsPerCore()
	// Fréquence effective depuis les jiffies /proc/stat × freq_max.
	// Même source que cpuUsage — cohérent, pas de relecture /proc/cpuinfo.
	// freq_eff_core[i] = (dActive[i]/dTotal[i]) × freqMax
	freqAvg := computeEffectiveFreq(t1, t2, freqMax)
	memTotal, memFree, memCached, memUsed := readMemFull()
	rxBytes, txBytes := readNetBytes()
	netIface, netIP := readMainIface()
	lbu := collectLBU()

	disks := collectDisks()
	loadAvg := collectLoadAvg()
	cpuTemps := collectCPUTemps()
	topProcs := collectTopProcs(20, memTotal*1024)

	// ── Échelles Y : initialisées à zéro ────────────────────────────
	// Le broker les remplace par les valeurs oscilloscope calculées
	// sur la fenêtre du ring buffer (computeOscilloScales).
	freqYMin := freqAvg - 300
	if freqYMin < 0 {
		freqYMin = 0
	}
	freqYMax := freqAvg + 300
	if freqMax > 0 && freqYMax > freqMax {
		freqYMax = freqMax
	}

	// ── Pics CPU : cores dont l'écart vs moyenne dépasse 20% ──────
	var peakCores []int
	for i, pct := range perCore {
		if mathAbs(pct-cpuUsage) > 20.0 {
			peakCores = append(peakCores, i)
		}
	}

	// ── Pourcentages mémoire pré-calculés ─────────────────────────
	var memUsedPct, memCachedPct uint8
	if memTotal > 0 {
		memUsedPct = uint8((memUsed * 100) / memTotal)
		memCachedPct = uint8((memCached * 100) / memTotal)
		if memUsedPct > 100 {
			memUsedPct = 100
		}
		if memCachedPct > 100 {
			memCachedPct = 100
		}
	}

	return Snapshot{
		Timestamp:    time.Now(),
		CPUUsage:     cpuUsage,
		CPUPerCore:   perCore,
		CPUFreqAvg:   freqAvg,
		CPUFreqMax:   freqMax,
		CPUFreqCore:  freqCore,
		CPUYMin:      0, // calculé par broker.computeOscilloScales
		CPUYMax:      0,
		CPUFreqYMin:  freqYMin,
		CPUFreqYMax:  freqYMax,
		MemYMin:      0,
		MemYMax:      0,
		CPUPeakCores: peakCores,
		MemTotal:     memTotal / 1024,
		MemFree:      memFree / 1024,
		MemUsed:      memUsed / 1024,
		MemCached:    memCached / 1024,
		// Valeurs cumulées pour empilement Chart.js (fill:origin)
		// Invariant : stackUsed + (stackCached-stackUsed) + (stackFree-stackCached) = total
		MemStackUsed:   memUsed / 1024,
		MemStackCached: (memUsed + memCached) / 1024,
		MemStackFree:   memTotal / 1024,
		MemUsedPct:     memUsedPct,
		MemCachedPct:   memCachedPct,
		NetRXBytes:     rxBytes,
		NetTXBytes:     txBytes,
		NetIface:       netIface,
		NetIP:          netIP,
		LBU:            lbu,
		Disks:          disks,
		LoadAvg:        loadAvg,
		CPUTemps:       cpuTemps,
		TopProcs:       topProcs,
	}
}

// ============================================================
// LBU — Alpine Linux Backup
// ============================================================

// collectLBU détecte l'état LBU via stat(2) uniquement.
//
// Algorithme :
//  1. /etc/apk/protected-paths.d inexistant → LBU absent (state:"absent")
//  2. Chercher la dernière *.tar.gz dans /var/cache/lbu/
//     • Aucune archive → state:"dirty" (jamais commité)
//     • Archive plus récente que protected-paths.d → state:"clean"
//     • Protected-paths.d plus récent que archive → state:"dirty"
//
// Zéro binaire externe, zéro exec — uniquement os.Stat + os.ReadDir.
func collectLBU() LBUStatus {
	const protectedPathsDir = "/etc/apk/protected-paths.d"
	const lbuCacheDir = "/var/cache/lbu"

	// ── 1. Présence de LBU ──────────────────────────────────────
	ppStat, err := os.Stat(protectedPathsDir)
	if err != nil {
		// Répertoire absent → LBU non installé
		return LBUStatus{Present: false, State: "absent"}
	}
	ppMtime := ppStat.ModTime()

	// ── 2. Dernière archive dans /var/cache/lbu/ ─────────────────
	entries, err := os.ReadDir(lbuCacheDir)
	if err != nil {
		// Cache inaccessible → traité comme dirty (jamais commité)
		return LBUStatus{Present: true, State: "dirty"}
	}

	var latestArchive string
	var latestMtime time.Time

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".tar.gz") {
			continue
		}
		info, err2 := e.Info()
		if err2 != nil {
			continue
		}
		if latestArchive == "" || info.ModTime().After(latestMtime) {
			latestArchive = name
			latestMtime = info.ModTime()
		}
	}

	// ── 3. Aucune archive → jamais commité ──────────────────────
	if latestArchive == "" {
		return LBUStatus{Present: true, State: "dirty"}
	}

	// ── 4. Comparaison mtime ─────────────────────────────────────
	// clean  : archive >= protected-paths.d
	// dirty  : protected-paths.d > archive  (modifications non persistées)
	if ppMtime.After(latestMtime) {
		return LBUStatus{Present: true, State: "dirty", Archive: latestArchive}
	}
	return LBUStatus{Present: true, State: "clean", Archive: latestArchive}
}

func mathAbs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// ============================================================
// DÉTECTION DE RÔLE HOST — TRINITY
// Compatible BusyBox/Alpine, zéro binaire externe, zéro exec.
// Uniquement /proc et /sys — disponibles sur tout kernel Linux.
//
// ORDRE DE PRIORITÉ (invariant) :
//   Container > Dom0 > DomU > Alpine baremetal
//
// Pourquoi container AVANT Xen :
//   Un process tournant dans Docker/Podman sur une DomU voit
//   /proc/xen (hérité du kernel Xen de la DomU) mais son contexte
//   réel est container. La détection container utilise des signaux
//   kernel indépendants de l'hyperviseur sous-jacent.
//
// Signaux utilisés (tous lisibles non-root) :
//   /proc/1/sched     → ligne 1 : "<comm> (<PID_HOST>, ...)"
//                       PID_HOST ≠ 1 = container prouvé (absolu)
//   /proc/1/cgroup    → chemin du sous-cgroup de PID 1
//                       "/docker/<id>", "libpod-<id>", "/lxc/..." → container
//   /.dockerenv       → fichier créé par Docker au démarrage
//   /run/.containerenv→ fichier créé par Podman
//   /proc/xen/capabilities → présence + "control_d" → Dom0/DomU
//   /sys/hypervisor/type   → "xen" → DomU (paravirt)
// ============================================================

// DetectHostRole identifie le rôle de l'hôte.
// Résultat mis en cache au démarrage du broker — appelé une seule fois.
func DetectHostRole() HostRole {
	// ── 1. Container ? (avant Xen — un container dans une DomU
	//       voit /proc/xen mais son rôle réel est container)
	if r, ok := probeContainer(); ok {
		return r
	}

	// ── 2. Xen Dom0 ou DomU ? (seulement si pas container)
	if r, ok := probeXen(); ok {
		return r
	}

	// ── 3. Alpine baremetal
	return HostRole{
		Role:     "Alpine",
		Runtime:  "native",
		Label:    "Alpine Linux · Baremetal",
		Verified: true,
	}
}

// ── probeContainer ─────────────────────────────────────────
//
// Trois niveaux de preuves, du plus absolu au plus fort.
// Premier match positif → retour immédiat.

func probeContainer() (HostRole, bool) {
	// Niveau A — Preuve absolue : /proc/1/sched
	//
	// Le kernel écrit dans ce fichier le PID du process dans le
	// namespace racine (PID initial = namespace du kernel, pas du container).
	// Format ligne 1 : "<comm> (<PID_NS_ROOT>, #threads: <n>)"
	//
	// Baremetal / VM / DomU sans container : PID_NS_ROOT == 1
	// Container (Docker, Podman, LXC...) : PID_NS_ROOT != 1
	//   ex. "sh (3421, #threads: 1)" → sched PID 3421 ≠ 1
	//
	// Ce signal est indépendant du runtime et de l'hyperviseur.
	// Il est présent même si /.dockerenv est absent.
	if schedPID, ok := readSchedPID("/proc/1/sched"); ok && schedPID != 1 {
		rt := identifyContainerRuntime()
		return HostRole{
			Role:     "Container",
			Runtime:  rt,
			Label:    containerLabel(rt),
			Verified: true,
		}, true
	}

	// Niveau B — Preuve forte : chemin cgroup de PID 1
	//
	// Sur baremetal/VM sans container : cgroup memory path = "/"
	// Dans un container : path = "/docker/<id>", "libpod-<id>", "/lxc/<id>", etc.
	//
	// On lit /proc/1/cgroup (pas /proc/self/cgroup) pour avoir le
	// cgroup de l'init du container, pas du process Go lui-même.
	if rt, ok := cgroupPath(); ok {
		return HostRole{
			Role:     "Container",
			Runtime:  rt,
			Label:    containerLabel(rt),
			Verified: true,
		}, true
	}

	// Niveau C — Preuve forte : marqueurs fichiers runtime
	//
	// Docker crée /.dockerenv dans chaque container au démarrage.
	// Podman crée /run/.containerenv (fichier INI avec métadonnées).
	// Ces fichiers peuvent être supprimés manuellement mais sont
	// présents dans 99% des déploiements standards.
	if fileExists("/run/.containerenv") {
		return HostRole{
			Role: "Container", Runtime: "podman",
			Label: "Container · Podman", Verified: true,
		}, true
	}
	if fileExists("/.dockerenv") {
		return HostRole{
			Role: "Container", Runtime: "docker",
			Label: "Container · Docker", Verified: true,
		}, true
	}

	return HostRole{}, false
}

// ── probeXen ───────────────────────────────────────────────

func probeXen() (HostRole, bool) {
	// Dom0 : /proc/xen/capabilities contient "control_d"
	// DomU : /proc/xen/capabilities existe mais sans "control_d"
	if data, err := os.ReadFile("/proc/xen/capabilities"); err == nil {
		if strings.Contains(string(data), "control_d") {
			return HostRole{
				Role: "Dom0", Runtime: "xen",
				Label: "Xen Dom0 · Hyperviseur", Verified: true,
			}, true
		}
		return HostRole{
			Role: "DomU", Runtime: "xen",
			Label: "Xen DomU · VM Alpine", Verified: true,
		}, true
	}

	// DomU paravirt : /sys/hypervisor/type == "xen"
	if data, err := os.ReadFile("/sys/hypervisor/type"); err == nil {
		if strings.EqualFold(strings.TrimSpace(string(data)), "xen") {
			return HostRole{
				Role: "DomU", Runtime: "xen",
				Label: "Xen DomU · VM Alpine", Verified: true,
			}, true
		}
	}

	return HostRole{}, false
}

// ── Helpers ────────────────────────────────────────────────

// readSchedPID lit /proc/<pid>/sched et retourne le PID dans le namespace racine.
// Ligne 1 format : "<comm> (<PID>, #threads: <n>)"
func readSchedPID(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	// Isoler la 1ʳᵉ ligne
	line := string(data)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	// Extraire le PID entre '(' et ','
	start := strings.IndexByte(line, '(')
	if start < 0 {
		return 0, false
	}
	rest := line[start+1:]
	comma := strings.IndexByte(rest, ',')
	if comma < 0 {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(rest[:comma]))
	if err != nil {
		return 0, false
	}
	return n, true
}

// cgroupPath lit /proc/1/cgroup et identifie le runtime container
// depuis les chemins de sous-cgroup.
//
// Cgroup v1 : chaque ligne = "<id>:<subsystem>:<path>"
//
//	ex. "3:memory:/docker/abc123"
//
// Cgroup v2 : ligne unique "0::/<path>"
//
//	ex. "0::/docker/abc123"
//
// Sur baremetal/DomU sans container : tous les paths = "/"
func cgroupPath() (string, bool) {
	data, err := os.ReadFile("/proc/1/cgroup")
	if err != nil {
		return "", false
	}
	// Concaténer tous les paths pour une seule passe de recherche
	content := strings.ToLower(string(data))

	// Ordre : du plus spécifique au plus générique
	switch {
	case strings.Contains(content, "libpod-"):
		return "podman", true
	case strings.Contains(content, "/podman/"), strings.Contains(content, "/podman-"):
		return "podman", true
	case strings.Contains(content, "/docker/"), strings.Contains(content, "/docker-"):
		return "docker", true
	case strings.Contains(content, "cri-containerd-"), strings.Contains(content, "kubepods"):
		return "containerd", true
	case strings.Contains(content, "/containerd-"):
		return "containerd", true
	case strings.Contains(content, "/lxc/"), strings.Contains(content, "/lxc-"):
		return "lxc", true
	case strings.Contains(content, "buildah-"):
		return "buildah", true
	}

	// Heuristique v2 : "0::/<path>" avec path non trivial
	// Baremetal : "0::/" ou "0::/init.scope" ou "0::/system.slice/init..."
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "0::") {
			continue
		}
		path := strings.TrimPrefix(line, "0::")
		if path == "" || path == "/" {
			return "", false
		}
		if strings.HasPrefix(path, "/init.scope") ||
			strings.HasPrefix(path, "/system.slice/init") ||
			strings.HasPrefix(path, "/user.slice/user-") {
			return "", false
		}
		// Chemin non trivial, runtime inconnu
		return "container", true
	}

	return "", false
}

// identifyContainerRuntime tente d'affiner le runtime quand sched prouve
// déjà qu'on est dans un container.
func identifyContainerRuntime() string {
	if rt, ok := cgroupPath(); ok {
		return rt
	}
	if fileExists("/run/.containerenv") {
		return "podman"
	}
	if fileExists("/.dockerenv") {
		return "docker"
	}
	if env := procEnvVar(1, "CONTAINER"); env != "" {
		return strings.ToLower(env)
	}
	return "container"
}

// procEnvVar lit la variable key dans /proc/<pid>/environ.
func procEnvVar(pid int, key string) string {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ")
	if err != nil {
		return ""
	}
	prefix := key + "="
	for _, entry := range strings.Split(string(data), "\x00") {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}

// fileExists retourne true si le chemin existe.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// containerLabel retourne le label UI pour un runtime.
func containerLabel(runtime string) string {
	switch runtime {
	case "docker":
		return "Container · Docker"
	case "podman":
		return "Container · Podman"
	case "containerd":
		return "Container · containerd"
	case "lxc":
		return "Container · LXC"
	case "buildah":
		return "Container · Buildah"
	case "systemd-nspawn":
		return "Container · systemd-nspawn"
	case "", "container":
		return "Container"
	}
	return "Container · " + runtime
}

// ============================================================
// CPU
// ============================================================

type cpuTimes struct{ idle, total uint64 }

// readCPPCMaxFreq lit la fréquence max turbo depuis ACPI CPPC.
// Sur Xen HWP (Dom0 PV), /sys/cpufreq est absent mais acpi_cppc est disponible.
// Formule : freq_max = (highest_perf / nominal_perf) × nominal_freq_MHz
// nominal_freq est en MHz dans /sys/devices/system/cpu/cpu0/acpi_cppc/nominal_freq

// readXenFreqMax lit la fréquence max turbo depuis xl dmesg.
// Xen expose "CPU0: bus: X MHz base: X MHz max: X MHz" sur les CPUs HWP.
// Si "max:" absent → CPU bridé, on retourne 0 (fallback /proc/cpuinfo).
func readXenFreqMax() int {
	out, err := os.ReadFile("/var/run/xen/xen-dmesg")
	if err != nil {
		// Essayer via commande xl dmesg
		cmd := exec.Command("xl", "dmesg")
		b, err2 := cmd.Output()
		if err2 != nil {
			return 0
		}
		out = b
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "CPU0:") && strings.Contains(line, "max:") {
			// Format: "(XEN) CPU0: bus: 100 MHz base: 1800 MHz max: 3800 MHz"
			if idx := strings.Index(line, "max:"); idx >= 0 {
				rest := strings.TrimSpace(line[idx+4:])
				parts := strings.Fields(rest)
				if len(parts) >= 1 {
					if v, err3 := strconv.Atoi(parts[0]); err3 == nil && v > 0 {
						return v
					}
				}
			}
		}
	}
	return 0
}

func readCPPCMaxFreq() int {
	base := "/sys/devices/system/cpu/cpu0/acpi_cppc/"
	highest := readIntFile(base + "highest_perf")
	nominal := readIntFile(base + "nominal_perf")
	nomFreq := readIntFile(base + "nominal_freq") // en MHz
	if highest > 0 && nominal > 0 && nomFreq > 0 {
		return int(float64(highest) / float64(nominal) * float64(nomFreq))
	}
	// Fallback : lire reference_perf
	refPerf := readIntFile(base + "reference_perf")
	if highest > 0 && refPerf > 0 && nomFreq > 0 {
		return int(float64(highest) / float64(refPerf) * float64(nomFreq))
	}
	return 0
}

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
		idle := vals[3] + vals[4]
		total := sum(vals[:8])
		out[fields[0]] = cpuTimes{idle: idle, total: total}
	}
	return out
}

// cpuPercentAll retourne l'usage global et un slice par core (ordre numérique).
func cpuPercentAll(t1, t2 map[string]cpuTimes) (float64, []float64) {
	type coreVal struct {
		idx int
		pct float64
	}
	var cores []coreVal
	var totalPct, count float64

	for name, a := range t1 {
		b, ok := t2[name]
		if !ok {
			continue
		}
		dIdle := float64(b.idle - a.idle)
		dTotal := float64(b.total - a.total)
		if dTotal <= 0 {
			continue
		}
		pct := (1 - dIdle/dTotal) * 100
		totalPct += pct
		count++

		// Extraire l'index numérique depuis "cpu0", "cpu1", ...
		idxStr := strings.TrimPrefix(name, "cpu")
		if idx, err := strconv.Atoi(idxStr); err == nil {
			cores = append(cores, coreVal{idx, pct})
		}
	}

	// Trier par index core
	for i := 1; i < len(cores); i++ {
		for j := i; j > 0 && cores[j].idx < cores[j-1].idx; j-- {
			cores[j], cores[j-1] = cores[j-1], cores[j]
		}
	}

	perCore := make([]float64, len(cores))
	for i, c := range cores {
		perCore[i] = c.pct
	}

	global := 0.0
	if count > 0 {
		global = totalPct / count
	}
	return global, perCore
}

// computeEffectiveFreq calcule la fréquence CPU effective depuis les jiffies /proc/stat.
// freq_eff = (dActive / dTotal) × freqMax — varie avec la charge réelle.
// Cohérent avec cpuPercentAll : même t1/t2, pas de relecture /proc.
func computeEffectiveFreq(t1, t2 map[string]cpuTimes, freqMax int) int {
	if freqMax <= 0 {
		return 0
	}
	var totalRatio float64
	var count int
	for name, a := range t1 {
		b, ok := t2[name]
		if !ok || !strings.HasPrefix(name, "cpu") || name == "cpu" {
			continue
		}
		dIdle := float64(b.idle - a.idle)
		dTotal := float64(b.total - a.total)
		if dTotal <= 0 {
			continue
		}
		active := 1.0 - dIdle/dTotal
		totalRatio += active
		count++
	}
	if count == 0 {
		return 0
	}
	return int(totalRatio / float64(count) * float64(freqMax))
}

func readFreqsPerCore() (avg, max int, perCore []int) {
	type coreFreq struct{ idx, mhz int }
	entries, err := os.ReadDir("/sys/devices/system/cpu/")
	if err == nil {
		var cf []coreFreq
		for _, e := range entries {
			name := e.Name()
			if !strings.HasPrefix(name, "cpu") || len(name) <= 3 {
				continue
			}
			idx, err2 := strconv.Atoi(strings.TrimPrefix(name, "cpu"))
			if err2 != nil {
				continue
			}
			base := "/sys/devices/system/cpu/" + name + "/cpufreq/"
			cur := readIntFile(base + "scaling_cur_freq")
			if cur == 0 {
				cur = readIntFile(base + "cpuinfo_cur_freq")
			}
			if cur > 0 {
				mhz := cur / 1000
				cf = append(cf, coreFreq{idx, mhz})
				if m := readIntFile(base + "cpuinfo_max_freq"); m/1000 > max {
					max = m / 1000
				}
			}
		}
		if len(cf) > 0 {
			for i := 1; i < len(cf); i++ {
				for j := i; j > 0 && cf[j].idx < cf[j-1].idx; j-- {
					cf[j], cf[j-1] = cf[j-1], cf[j]
				}
			}
			s := 0
			for _, c := range cf {
				s += c.mhz
				perCore = append(perCore, c.mhz)
			}
			avg = s / len(cf)
			return
		}
	}
	// Fallback /proc/cpuinfo
	data, _ := os.ReadFile("/proc/cpuinfo")
	var freqs []int
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "cpu MHz") {
			if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
				if f, err3 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err3 == nil {
					freqs = append(freqs, int(f))
					if int(f) > max {
						max = int(f)
					}
				}
			}
		}
	}
	// Fallback max turbo via xl dmesg (Xen HWP/bridé)
	// "CPU0: bus: X MHz base: X MHz max: X MHz" → max turbo
	// Si "max:" absent → CPU bridé, freqMax reste valeur /proc/cpuinfo
	xenMax := readXenFreqMax()
	if xenMax > max {
		max = xenMax
	}

	if len(freqs) > 0 {
		s := 0
		for _, f := range freqs {
			s += f
			perCore = append(perCore, f)
		}
		avg = s / len(freqs)
	}
	return
}

// ============================================================
// MÉMOIRE
// ============================================================

// readMemFull retourne les composantes mémoire selon la formule htop.
//
//	used   = MemTotal - MemFree - Buffers - Cached  (vraiement utilisée)
//	cached = Buffers + Cached
//	free   = MemFree
//
// Invariant garanti : used + cached + free == total (en kB).
func readMemFull() (total, free, cached, used uint64) {
	data, _ := os.ReadFile("/proc/meminfo")
	var buffers, cachedRaw uint64
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			total = parseKB(line)
		case strings.HasPrefix(line, "MemFree:"):
			free = parseKB(line)
		case strings.HasPrefix(line, "Buffers:"):
			buffers = parseKB(line)
		case strings.HasPrefix(line, "Cached:"):
			// "Cached:" uniquement — évite "SwapCached:"
			if line[6] == ':' || line[6] == ' ' {
				cachedRaw = parseKB(line)
			}
		}
	}
	cached = buffers + cachedRaw
	if total > free+cached {
		used = total - free - cached
	}
	return
}

// ============================================================
// RÉSEAU
// ============================================================

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

func readMainIface() (iface, ip string) {
	data, _ := os.ReadFile("/proc/net/dev")
	var bestRx uint64
	for _, line := range strings.Split(string(data), "\n")[2:] {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		name := strings.Trim(fields[0], ":")
		if name == "lo" {
			continue
		}
		rx := parseU64(fields[1])
		if iface == "" || rx > bestRx {
			bestRx = rx
			iface = name
		}
	}
	if iface == "" {
		return
	}
	ifaces, _ := net.Interfaces()
	for _, itf := range ifaces {
		if itf.Name != iface {
			continue
		}
		addrs, _ := itf.Addrs()
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				ip = ipnet.IP.String()
				return
			}
		}
	}
	return
}

// ============================================================
// INFOS STATIQUES HW/OS
// ============================================================

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
	if pretty == "" {
		pretty = "unknown"
	}
	if version == "" {
		version = "unknown"
	}
	return
}

// ============================================================
// HELPERS GÉNÉRIQUES
// ============================================================

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

// readFirstFile retourne le contenu du premier fichier lisible parmi ceux fournis.
func readFirstFile(paths ...string) string {
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

// readFirstFileTrim : alias de readFirstFile (rétrocompat interne).
func readFirstFileTrim(paths ...string) string {
	return readFirstFile(paths...)
}

// ============================================================
// NETWORK MAP — topologie + voisins ARP
// ============================================================

// NetIfaceInfo décrit une interface réseau avec ses métriques temps réel.
// Lecture de /proc/net/dev — zéro binaire externe.
type NetIfaceInfo struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`
	RXBytes uint64 `json:"rx_bytes"`
	TXBytes uint64 `json:"tx_bytes"`
	RXBps   uint64 `json:"rx_bps"`
	TXBps   uint64 `json:"tx_bps"`
	Up      bool   `json:"up"`
}

// ARPEntry décrit un voisin réseau connu via la table ARP.
// Lecture de /proc/net/arp — zéro binaire externe.
type ARPEntry struct {
	IP    string `json:"ip"`
	MAC   string `json:"mac"`
	Iface string `json:"iface"`
	State string `json:"state"` // "reachable" | "stale" | "unknown"
}

// NetworkMap regroupe les interfaces et les voisins pour la topologie.
type NetworkMap struct {
	Interfaces []NetIfaceInfo `json:"interfaces"`
	Neighbors  []ARPEntry     `json:"neighbors"`
}

// collectNetworkMap lit /proc/net/dev et /proc/net/arp.
// Les débits (bps) nécessitent deux relevés — on passe l'ancien snapshot
// pour calculer le delta. Si prev est nil, bps = 0.
func collectNetworkMap(prevIfaces map[string]NetIfaceInfo, interval float64) (NetworkMap, map[string]NetIfaceInfo) {
	ifaces := readAllIfaces()
	neighbors := readARPTable()

	// Calcul des débits par interface
	current := make(map[string]NetIfaceInfo, len(ifaces))
	for i, iface := range ifaces {
		current[iface.Name] = iface
		if prevIfaces != nil && interval > 0 {
			if prev, ok := prevIfaces[iface.Name]; ok {
				if iface.RXBytes >= prev.RXBytes {
					ifaces[i].RXBps = uint64(float64(iface.RXBytes-prev.RXBytes) / interval)
				}
				if iface.TXBytes >= prev.TXBytes {
					ifaces[i].TXBps = uint64(float64(iface.TXBytes-prev.TXBytes) / interval)
				}
			}
		}
	}

	return NetworkMap{Interfaces: ifaces, Neighbors: neighbors}, current
}

// readAllIfaces lit /proc/net/dev et retourne toutes les interfaces (sauf lo).
func readAllIfaces() []NetIfaceInfo {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return nil
	}

	// Récupérer les IPs via net.Interfaces() une seule fois
	ipMap := make(map[string]string)
	upMap := make(map[string]bool)
	if netIfaces, err2 := net.Interfaces(); err2 == nil {
		for _, itf := range netIfaces {
			upMap[itf.Name] = itf.Flags&net.FlagUp != 0
			addrs, _ := itf.Addrs()
			for _, a := range addrs {
				if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil {
					ipMap[itf.Name] = ipnet.IP.String()
					break
				}
			}
		}
	}

	var result []NetIfaceInfo
	lines := strings.Split(string(data), "\n")
	if len(lines) < 3 {
		return nil
	}
	for _, line := range lines[2:] {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		name := strings.Trim(fields[0], ":")
		if name == "lo" {
			continue
		}
		result = append(result, NetIfaceInfo{
			Name:    name,
			IP:      ipMap[name],
			RXBytes: parseU64(fields[1]),
			TXBytes: parseU64(fields[9]),
			Up:      upMap[name],
		})
	}
	return result
}

// readARPTable lit /proc/net/arp.
// Format : IP address / HW type / Flags / HW address / Mask / Device
// Flags : 0x2 = reachable, 0x4 = stale, 0x6 = reachable+stale (approximation)
func readARPTable() []ARPEntry {
	data, err := os.ReadFile("/proc/net/arp")
	if err != nil {
		return nil
	}

	var result []ARPEntry
	lines := strings.Split(string(data), "\n")
	if len(lines) < 2 {
		return nil
	}
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		ip := fields[0]
		flags := fields[2]
		mac := fields[3]
		iface := fields[5]

		// Ignorer les entrées incomplètes (MAC = 00:00:00:00:00:00)
		if mac == "00:00:00:00:00:00" {
			continue
		}

		state := "unknown"
		if flags == "0x2" {
			state = "reachable"
		} else if flags == "0x4" || flags == "0x6" {
			state = "stale"
		}

		result = append(result, ARPEntry{
			IP:    ip,
			MAC:   mac,
			Iface: iface,
			State: state,
		})
	}
	return result
}

// ============================================================
// DISK — espaces disque via /proc/mounts + syscall.Statfs
// ============================================================

// DiskMount décrit un point de montage avec son espace disque.
// Zéro binaire externe — syscall.Statfs uniquement.
type DiskMount struct {
	Device  string `json:"device"`
	Mount   string `json:"mount"`
	FSType  string `json:"fstype"`
	Total   uint64 `json:"total_mb"`
	Used    uint64 `json:"used_mb"`
	Free    uint64 `json:"free_mb"`
	UsedPct uint8  `json:"used_pct"`
	Pal     string `json:"pal"`
	BarCls  string `json:"bar_cls"`
}

// collectDisks lit /proc/mounts et retourne un disque par device physique.
// Stratégie 2 passes : passe 1 → trouver le mount le plus court par device,
// passe 2 → Statfs + calculs uniquement sur ce mount.
func collectDisks() []DiskMount {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil
	}

	skipFS := map[string]bool{
		"proc": true, "sysfs": true, "devtmpfs": true, "devpts": true,
		"cgroup": true, "cgroup2": true, "pstore": true, "bpf": true,
		"tracefs": true, "debugfs": true, "securityfs": true, "hugetlbfs": true,
		"mqueue": true, "fusectl": true, "rpc_pipefs": true, "nfsd": true,
		"autofs": true, "configfs": true, "efivarfs": true, "selinuxfs": true,
		"tmpfs": true, "overlay": true, "squashfs": true,
	}

	isPhysicalDev := func(dev string) bool {
		return strings.HasPrefix(dev, "/dev/sd") ||
			strings.HasPrefix(dev, "/dev/xvd") ||
			strings.HasPrefix(dev, "/dev/vd") ||
			strings.HasPrefix(dev, "/dev/nvme") ||
			strings.HasPrefix(dev, "/dev/hd") ||
			strings.HasPrefix(dev, "/dev/mmcblk")
	}

	type entry struct{ mount, fstype string }
	best := make(map[string]entry) // device → entry avec mount le plus court

	seenMount := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		device, mount, fstype := fields[0], fields[1], fields[2]
		if skipFS[fstype] || !isPhysicalDev(device) {
			continue
		}
		if seenMount[mount] {
			continue
		}
		seenMount[mount] = true
		if prev, ok := best[device]; !ok || len(mount) < len(prev.mount) {
			best[device] = entry{mount, fstype}
		}
	}

	pals := []string{"res-cyan", "res-amber", "res-steel", "res-violet", "res-lime", "res-rose"}
	var result []DiskMount

	for device, e := range best {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(e.mount, &stat); err != nil {
			continue
		}
		bsize := uint64(stat.Bsize)
		total := stat.Blocks * bsize / 1024 / 1024
		free := stat.Bavail * bsize / 1024 / 1024
		used := total - (stat.Bfree * bsize / 1024 / 1024)
		if total == 0 {
			continue
		}
		pct := uint8((used * 100) / total)
		pal := pals[len(result)%len(pals)]
		barCls := ""
		if pct > 75 {
			barCls = "crit"
		} else if pct > 50 {
			barCls = "warn"
		}
		result = append(result, DiskMount{
			Device:  device,
			Mount:   e.mount,
			FSType:  e.fstype,
			Total:   total,
			Used:    used,
			Free:    free,
			UsedPct: pct,
			Pal:     pal,
			BarCls:  barCls,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Device != result[j].Device {
			return naturalLess(result[i].Device, result[j].Device)
		}
		return naturalLess(result[i].Mount, result[j].Mount)
	})
	return result
}

func naturalLess(a, b string) bool {
	ia, ib := 0, 0
	for ia < len(a) && ib < len(b) {
		ca, cb := a[ia], b[ib]
		if isDigit(ca) && isDigit(cb) {
			na, enda := readUintChunk(a, ia)
			nb, endb := readUintChunk(b, ib)
			if na != nb {
				return na < nb
			}
			ia, ib = enda, endb
			continue
		}
		if ca != cb {
			return ca < cb
		}
		ia++
		ib++
	}
	return len(a) < len(b)
}

func readUintChunk(s string, start int) (uint64, int) {
	i := start
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	n, _ := strconv.ParseUint(s[start:i], 10, 64)
	return n, i
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// ============================================================
// LOAD AVERAGE — /proc/loadavg
// ============================================================

// LoadAverage contient les moyennes de charge système (1, 5, 15 minutes).
type LoadAverage struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

// collectLoadAvg lit /proc/loadavg.
func collectLoadAvg() LoadAverage {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return LoadAverage{}
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return LoadAverage{}
	}
	parse := func(s string) float64 {
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}
	return LoadAverage{
		Load1:  parse(fields[0]),
		Load5:  parse(fields[1]),
		Load15: parse(fields[2]),
	}
}

// ============================================================
// CPU TEMPERATURES — /sys/class/thermal
// ============================================================

// CPUTemp décrit une zone thermique.
type CPUTemp struct {
	Zone  string  `json:"zone"`
	Label string  `json:"label"`
	TempC float64 `json:"temp_c"`
}

// collectCPUTemps lit toutes les zones thermiques disponibles.
// Zéro binaire externe — lecture directe de /sys/class/thermal.
func collectCPUTemps() []CPUTemp {
	entries, err := os.ReadDir("/sys/class/thermal")
	if err != nil {
		return nil
	}

	var result []CPUTemp
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "thermal_zone") {
			continue
		}
		base := "/sys/class/thermal/" + e.Name()

		// Lire la température (en millièmes de degrés Celsius)
		rawTemp, err := os.ReadFile(base + "/temp")
		if err != nil {
			continue
		}
		milli, err := strconv.ParseInt(strings.TrimSpace(string(rawTemp)), 10, 64)
		if err != nil || milli <= 0 {
			continue
		}

		// Lire le type/label
		label := e.Name()
		if raw, err2 := os.ReadFile(base + "/type"); err2 == nil {
			t := strings.TrimSpace(string(raw))
			if t != "" {
				label = t
			}
		}

		result = append(result, CPUTemp{
			Zone:  e.Name(),
			Label: label,
			TempC: float64(milli) / 1000.0,
		})
	}
	return result
}

// ============================================================
// TOP PROCESSES — /proc/*/status + /proc/*/stat
// ============================================================

// ProcInfo décrit un processus.
type ProcInfo struct {
	PID    int     `json:"pid"`
	Name   string  `json:"name"`
	State  string  `json:"state"`
	CPUPct float64 `json:"cpu_pct"`
	MemMB  uint64  `json:"mem_mb"`
	MemPct float64 `json:"mem_pct"`
	Virt   string  `json:"virt"`
	Res    string  `json:"res"`
	Shr    string  `json:"shr"`
	User   string  `json:"user"`
	Cmd    string  `json:"cmd"`
}

// collectTopProcs lit /proc et retourne les top N processus par mémoire RSS.
// Enrichit chaque entrée avec VmVirt, VmRSS, VmShr, Cmd, MemPct.
func collectTopProcs(n int, memTotalKB uint64) []ProcInfo {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	var procs []ProcInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		base := "/proc/" + e.Name()

		statusData, err := os.ReadFile(base + "/status")
		if err != nil {
			continue
		}

		var name, state, vmRSS, vmVirt, vmShr string
		var uid string
		for _, line := range strings.Split(string(statusData), "\n") {
			switch {
			case strings.HasPrefix(line, "Name:"):
				name = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
			case strings.HasPrefix(line, "State:"):
				f := strings.Fields(line)
				if len(f) >= 2 {
					state = f[1]
				}
			case strings.HasPrefix(line, "VmRSS:"):
				f := strings.Fields(line)
				if len(f) >= 2 {
					vmRSS = f[1]
				}
			case strings.HasPrefix(line, "VmSize:"):
				f := strings.Fields(line)
				if len(f) >= 2 {
					vmVirt = f[1]
				}
			case strings.HasPrefix(line, "RsShmem:"), strings.HasPrefix(line, "VmShr:"):
				f := strings.Fields(line)
				if len(f) >= 2 {
					vmShr = f[1]
				}
			case strings.HasPrefix(line, "Uid:"):
				f := strings.Fields(line)
				if len(f) >= 2 {
					uid = f[1]
				}
			}
		}

		rssKB, _ := strconv.ParseUint(vmRSS, 10, 64)
		if rssKB == 0 || name == "" {
			continue
		}

		// Commande depuis /proc/PID/cmdline (truncated)
		cmd := name
		if cmdData, err := os.ReadFile(base + "/cmdline"); err == nil && len(cmdData) > 0 {
			// cmdline est nul-séparé
			for i, b := range cmdData {
				if b == 0 {
					cmdData[i] = ' '
				}
			}
			cmdStr := strings.TrimSpace(string(cmdData))
			if len(cmdStr) > 60 {
				cmdStr = cmdStr[:60]
			}
			if cmdStr != "" {
				cmd = cmdStr
			}
		}

		virtKB, _ := strconv.ParseUint(vmVirt, 10, 64)
		shrKB, _ := strconv.ParseUint(vmShr, 10, 64)

		var memPct float64
		if memTotalKB > 0 {
			memPct = math.Round(float64(rssKB)/float64(memTotalKB)*1000) / 10
		}

		// Résolution de l'UID → nom (best-effort depuis /etc/passwd)
		userName := uid
		if uid == "0" {
			userName = "root"
		}

		procs = append(procs, ProcInfo{
			PID:    pid,
			Name:   name,
			State:  state,
			MemMB:  rssKB / 1024,
			MemPct: memPct,
			Virt:   fmtKB(virtKB),
			Res:    fmtKB(rssKB),
			Shr:    fmtKB(shrKB),
			User:   userName,
			Cmd:    cmd,
		})
	}

	// Trier par RSS décroissant
	for i := 0; i < len(procs); i++ {
		for j := i + 1; j < len(procs); j++ {
			if procs[j].MemMB > procs[i].MemMB {
				procs[i], procs[j] = procs[j], procs[i]
			}
		}
	}
	if len(procs) > n {
		procs = procs[:n]
	}
	return procs
}

// fmtKB formate un nombre de kilobytes en chaîne lisible (K/M/G).
func fmtKB(kb uint64) string {
	if kb >= 1024*1024 {
		return strconv.FormatUint(kb/1024/1024, 10) + "G"
	}
	if kb >= 1024 {
		return strconv.FormatUint(kb/1024, 10) + "M"
	}
	return strconv.FormatUint(kb, 10) + "K"
}
