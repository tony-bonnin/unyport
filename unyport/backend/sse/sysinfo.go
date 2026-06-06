package sse

// sysinfo.go — TRINITY · portage ACF Lua health-model.lua
//
// Fonctionnalités portées depuis Lua (health-model.lua) :
//   • BIOS : vendor, version, date         → /sys/devices/virtual/dmi/id/bios_*
//   • Board complémentaire : version        → /sys/devices/virtual/dmi/id/board_version
//   • Kernel modules (listing /proc/modules) → zéro exec
//   • GPU info                               → /sys/class/drm/ + /sys/bus/pci/devices/*/class
//
// Paradigme TRINITY : zéro binaire externe, zéro exec, /proc+/sys uniquement.
// Lua utilisait cat/fdisk/lspci via io.popen — tout remplacé par lecture directe.
//
// Endpoints HTTP exposés (tous protégés operator+ en amont dans routes.go) :
//
//   GET /api/bios         → BIOSInfo + board_version
//   GET /api/modules      → liste des kernel modules (/proc/modules)
//   GET /api/gpus         → GPUs détectés (/sys/class/drm + /sys/bus/pci)
//   GET /api/packages     → paquets APK installés (/lib/apk/db/installed)
//   GET /api/services     → services OpenRC + état (/etc/runlevels + /run/openrc)
//   GET /api/logs         → liste des fichiers de log disponibles
//   GET /api/logs/tail    → dernières N lignes d'un fichier (?file=messages&n=100)

import (
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// ============================================================
// BIOS / Firmware Info — /sys/devices/virtual/dmi/id/
// ============================================================

// BIOSInfo contient les informations firmware de la carte.
// Absent sur machines virtuelles (champs vides) ou containers.
// Source : DMI table exposée via sysfs — lisible sans root.
type BIOSInfo struct {
	Vendor  string `json:"bios_vendor"`  // ex. "American Megatrends Inc."
	Version string `json:"bios_version"` // ex. "F.70"
	Date    string `json:"bios_date"`    // ex. "08/25/2023"
	// Champ additionnel : année extraite de bios_date (utile pour l'UI)
	Year string `json:"bios_year"` // ex. "2023" — extrait depuis bios_date
}

// BoardInfoExtra complète les données déjà présentes dans SystemInfo.
// SystemInfo expose déjà board_name et board_vendor ;
// ce struct ajoute board_version qui était dans health-model.lua
// mais manquait dans SystemInfoHandler.
type BoardInfoExtra struct {
	Version string `json:"board_version"` // ex. "Rev X.0"
}

// CollectBIOSInfo lit les infos BIOS depuis le sysfs DMI.
// Retourne une struct vide (champs "") si les fichiers sont absents
// (cas DomU, container) — l'UI masque la section dans ce cas.
func CollectBIOSInfo() BIOSInfo {
	b := BIOSInfo{
		Vendor:  trim(readFirstFile("/sys/devices/virtual/dmi/id/bios_vendor")),
		Version: trim(readFirstFile("/sys/devices/virtual/dmi/id/bios_version")),
		Date:    trim(readFirstFile("/sys/devices/virtual/dmi/id/bios_date")),
	}
	b.Year = extractBIOSYear(b.Date)
	return b
}

// CollectBoardVersion lit le champ board_version manquant dans SystemInfo.
func CollectBoardVersion() string {
	return trim(readFirstFile("/sys/devices/virtual/dmi/id/board_version"))
}

// extractBIOSYear tente d'extraire l'année depuis une date BIOS.
// Formats courants : "MM/DD/YYYY" (AMI), "YYYY-MM-DD" (UEFI), "DD/MM/YYYY" (EU).
// Retourne "" si la date est vide ou non parseable.
func extractBIOSYear(date string) string {
	if date == "" {
		return ""
	}
	if len(date) >= 4 && date[4] == '-' {
		if y := date[:4]; isYear(y) {
			return y
		}
	}
	parts := strings.Split(date, "/")
	if len(parts) == 3 {
		last := parts[2]
		if sp := strings.Index(last, " "); sp > 0 {
			last = last[:sp]
		}
		if isYear(last) {
			return last
		}
	}
	return ""
}

func isYear(s string) bool {
	if len(s) != 4 {
		return false
	}
	n, err := strconv.Atoi(s)
	return err == nil && n >= 1990 && n <= 2100
}

// ── /api/bios ────────────────────────────────────────────────────────────────

// BIOSHandler retourne les infos BIOS + board_version.
// Champs vides si la machine est une VM sans DMI table (DomU paravirt, container).
//
// Portage de : health-model.lua → biosVendor, biosVersion, biosDate, boardVersion
func (b *Broker) BIOSHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type biosResponse struct {
		BIOS         BIOSInfo `json:"bios"`
		BoardVersion string   `json:"board_version"`
		Available    bool     `json:"available"`
	}

	bios := CollectBIOSInfo()
	boardVer := CollectBoardVersion()
	available := bios.Vendor != "" || bios.Version != ""

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(biosResponse{
		BIOS:         bios,
		BoardVersion: boardVer,
		Available:    available,
	})
}

// ============================================================
// KERNEL MODULES — /proc/modules
// ============================================================

// KernelModule décrit un module chargé.
type KernelModule struct {
	Name     string `json:"name"`      // ex. "xen_blkfront"
	Size     int64  `json:"size"`      // octets en mémoire
	UseCount int    `json:"use_count"` // référencé par N modules
	UsedBy   string `json:"used_by"`   // liste "-" si vide
	State    string `json:"state"`     // "Live" | "Loading" | "Unloading"
	Offset   string `json:"offset"`    // adresse mémoire kernel (hex)
}

// CollectKernelModules lit /proc/modules et retourne tous les modules chargés.
// Format d'une ligne :
//
//	<name> <size> <use_count> <used_by> <state> <offset>
//	ex. xen_blkfront 45056 0 - Live 0xffffffffc0a00000
//
// Zéro exec — pas de lsmod, pas de modinfo.
// Paradigme ACF original (modules-model.lua) utilisait lsmod (binaire).
func CollectKernelModules() []KernelModule {
	data, err := os.ReadFile("/proc/modules")
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	result := make([]KernelModule, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		mod := KernelModule{Name: fields[0]}
		mod.Size, _ = strconv.ParseInt(fields[1], 10, 64)
		mod.UseCount, _ = strconv.Atoi(fields[2])
		if len(fields) >= 4 {
			mod.UsedBy = fields[3]
		}
		if len(fields) >= 5 {
			mod.State = fields[4]
		}
		if len(fields) >= 6 {
			mod.Offset = fields[5]
		}
		result = append(result, mod)
	}

	return result
}

// ModulesSummary est une version compacte pour l'API (count + liste des noms).
type ModulesSummary struct {
	Count   int      `json:"count"`
	Modules []string `json:"modules"` // noms uniquement, triés
}

// CollectModulesSummary retourne le résumé compact (nombre + noms).
func CollectModulesSummary() ModulesSummary {
	mods := CollectKernelModules()
	names := make([]string, len(mods))
	for i, m := range mods {
		names[i] = m.Name
	}
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	return ModulesSummary{Count: len(mods), Modules: names}
}

// ── /api/modules ─────────────────────────────────────────────────────────────

// ModulesHandler retourne la liste complète des kernel modules chargés.
// Portage de : modules-model.lua (utilisait lsmod — binaire)
// Source : /proc/modules — zéro exec.
//
// Query params :
//
//	?summary=1  → retourne seulement le count + liste des noms (plus léger)
func (b *Broker) ModulesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if r.URL.Query().Get("summary") == "1" {
		_ = json.NewEncoder(w).Encode(CollectModulesSummary())
		return
	}

	modules := CollectKernelModules()
	if modules == nil {
		modules = []KernelModule{}
	}

	type modulesResponse struct {
		Count   int            `json:"count"`
		Modules []KernelModule `json:"modules"`
	}
	_ = json.NewEncoder(w).Encode(modulesResponse{
		Count:   len(modules),
		Modules: modules,
	})
}

// ============================================================
// GPU INFO — /sys/class/drm/ + /sys/bus/pci/devices/*/class
// ============================================================

// GPUInfo décrit un GPU détecté.
// Lua utilisait lspci (binaire root requis).
// Ici : /sys/class/drm/ pour les GPUs actifs + /sys/bus/pci/devices/
// pour identifier les classes PCI 0x03xx (display controller).
//
// Lisible sans root — sysfs expose ces infos à tous les utilisateurs.
type GPUInfo struct {
	Name    string `json:"name"`     // ex. "Intel UHD Graphics 620"
	Driver  string `json:"driver"`   // ex. "i915" | "amdgpu" | "nouveau"
	PCIAddr string `json:"pci_addr"` // ex. "0000:00:02.0"
	Vendor  string `json:"vendor"`   // ex. "Intel Corporation" ou code vendeur hex
	Present bool   `json:"present"`  // false = pas de GPU détecté
}

// CollectGPUs détecte les GPUs via /sys/class/drm/.
func CollectGPUs() []GPUInfo {
	var result []GPUInfo
	seen := map[string]bool{}

	// ── Méthode 1 : /sys/class/drm/ ──────────────────────────────
	entries, err := os.ReadDir("/sys/class/drm")
	if err == nil {
		for _, e := range entries {
			name := e.Name()
			if !isCardEntry(name) {
				continue
			}
			base := "/sys/class/drm/" + name + "/device"
			pciAddr := extractPCIFromUevent(base + "/uevent")
			if pciAddr == "" || seen[pciAddr] {
				continue
			}
			seen[pciAddr] = true

			gpu := GPUInfo{PCIAddr: pciAddr, Present: true}
			vendorHex := trim(readFirstFile(base + "/vendor"))
			gpu.Vendor = resolveVendor(vendorHex)

			driverLink, lerr := os.Readlink(base + "/driver")
			if lerr == nil {
				gpu.Driver = trimPath(driverLink)
			}

			deviceHex := trim(readFirstFile(base + "/device"))
			gpu.Name = resolveGPUName(gpu.Vendor, deviceHex, gpu.Driver)
			result = append(result, gpu)
		}
	}

	// ── Méthode 2 : /sys/bus/pci/devices/ (fallback) ─────────────
	pciDevs, perr := os.ReadDir("/sys/bus/pci/devices")
	if perr == nil {
		for _, e := range pciDevs {
			addr := e.Name()
			if seen[addr] {
				continue
			}
			classRaw := trim(readFirstFile("/sys/bus/pci/devices/" + addr + "/class"))
			if !isPCIDisplayClass(classRaw) {
				continue
			}
			seen[addr] = true
			vendorHex := trim(readFirstFile("/sys/bus/pci/devices/" + addr + "/vendor"))
			deviceHex := trim(readFirstFile("/sys/bus/pci/devices/" + addr + "/device"))
			vendor := resolveVendor(vendorHex)
			result = append(result, GPUInfo{
				PCIAddr: addr,
				Vendor:  vendor,
				Name:    resolveGPUName(vendor, deviceHex, ""),
				Driver:  "",
				Present: true,
			})
		}
	}

	return result
}

// ── /api/gpus ────────────────────────────────────────────────────────────────

// GPUsHandler retourne les GPUs détectés.
// Portage de : health-model.lua → get_proc() utilisait lspci (binaire root)
func (b *Broker) GPUsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	gpus := CollectGPUs()
	if gpus == nil {
		gpus = []GPUInfo{}
	}

	type gpuResponse struct {
		Count int       `json:"count"`
		GPUs  []GPUInfo `json:"gpus"`
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(gpuResponse{Count: len(gpus), GPUs: gpus})
}

// ============================================================
// APK DATABASE — /lib/apk/db/installed
// ============================================================

// APKPackage décrit un paquet Alpine installé.
// Lua utilisait `apk` (binaire) — ici on parse directement la DB.
// Format : sections séparées par ligne vide, champs "K:value\n".
type APKPackage struct {
	Name    string `json:"name"`    // ex. "musl"
	Version string `json:"version"` // ex. "1.2.4-r2"
	Arch    string `json:"arch"`    // ex. "x86_64"
	Size    int64  `json:"size"`    // taille installée en octets
	Desc    string `json:"desc"`    // description courte
}

// CollectAPKPackages parse /lib/apk/db/installed et retourne les paquets installés.
//
// Format de la DB APK :
//
//	P:package-name
//	V:1.2.3-r0
//	A:x86_64
//	I:65536        (installed size, octets)
//	T:Short description
//	(ligne vide = séparateur de paquet)
//
// Zéro exec — pas de `apk info`, pas de `apk list`.
func CollectAPKPackages() []APKPackage {
	data, err := os.ReadFile("/lib/apk/db/installed")
	if err != nil {
		return nil
	}

	var result []APKPackage
	var cur APKPackage

	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			if cur.Name != "" {
				result = append(result, cur)
				cur = APKPackage{}
			}
			continue
		}
		if len(line) < 2 || line[1] != ':' {
			continue
		}
		key := line[0]
		val := line[2:]
		switch key {
		case 'P':
			cur.Name = val
		case 'V':
			cur.Version = val
		case 'A':
			cur.Arch = val
		case 'I':
			cur.Size, _ = strconv.ParseInt(val, 10, 64)
		case 'T':
			cur.Desc = val
		}
	}
	if cur.Name != "" {
		result = append(result, cur)
	}
	return result
}

// APKSummary est la vue compacte pour l'API.
type APKSummary struct {
	Count    int  `json:"count"`
	DBExists bool `json:"db_exists"`
}

// CollectAPKSummary retourne le résumé sans charger tous les paquets.
func CollectAPKSummary() APKSummary {
	_, err := os.Stat("/lib/apk/db/installed")
	if err != nil {
		return APKSummary{DBExists: false}
	}
	pkgs := CollectAPKPackages()
	return APKSummary{Count: len(pkgs), DBExists: true}
}

// ── /api/packages ────────────────────────────────────────────────────────────

// PackagesHandler retourne les paquets APK installés.
// Portage de : apk-model.lua (utilisait `apk` binaire pour tout)
//
// Query params :
//
//	?summary=1         → count seulement
//	?filter=<prefix>   → filtre par préfixe de nom
func (b *Broker) PackagesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if r.URL.Query().Get("summary") == "1" {
		_ = json.NewEncoder(w).Encode(CollectAPKSummary())
		return
	}

	pkgs := CollectAPKPackages()
	if pkgs == nil {
		pkgs = []APKPackage{}
	}

	if filter := r.URL.Query().Get("filter"); filter != "" {
		filtered := pkgs[:0]
		for _, p := range pkgs {
			if len(p.Name) >= len(filter) && p.Name[:len(filter)] == filter {
				filtered = append(filtered, p)
			}
		}
		pkgs = filtered
	}

	type packagesResponse struct {
		Count    int          `json:"count"`
		DBExists bool         `json:"db_exists"`
		Packages []APKPackage `json:"packages"`
	}
	_ = json.NewEncoder(w).Encode(packagesResponse{
		Count:    len(pkgs),
		DBExists: true,
		Packages: pkgs,
	})
}

// ============================================================
// OPENRC SERVICES — /etc/runlevels/ + /run/openrc/
// ============================================================

// ServiceState représente l'état d'un service OpenRC.
type ServiceState string

const (
	ServiceStarted  ServiceState = "started"
	ServiceStopped  ServiceState = "stopped"
	ServiceCrashed  ServiceState = "crashed"
	ServiceInactive ServiceState = "inactive"
)

// OpenRCService décrit un service OpenRC.
type OpenRCService struct {
	Name       string       `json:"name"`       // ex. "sshd"
	State      ServiceState `json:"state"`      // started | stopped | crashed | inactive
	Runlevels  []string     `json:"runlevels"`  // runlevels où il est activé
	Supervised bool         `json:"supervised"` // true si /run/openrc/started/<name> présent
}

// CollectOpenRCServices liste tous les services OpenRC et leur état.
//
// Sources utilisées (zéro exec, zéro rc-status binaire) :
//
//	/etc/runlevels/<level>/<service> → activation par runlevel (symlinks)
//	/run/openrc/started/<service>    → service démarré et supervisé
//	/run/openrc/exclusive/<service>  → service en mode exclusif
//	/etc/init.d/                     → liste exhaustive des services disponibles
//
// Lua utilisait processinfo.daemoncontrol() + read_initrunlevels() qui
// appelaient /sbin/rc-status (binaire). On évite totalement les binaires.
func CollectOpenRCServices() []OpenRCService {
	svcRunlevels := map[string][]string{}

	levels, err := os.ReadDir("/etc/runlevels")
	if err == nil {
		for _, lvl := range levels {
			if !lvl.IsDir() {
				continue
			}
			lvlName := lvl.Name()
			entries, eerr := os.ReadDir("/etc/runlevels/" + lvlName)
			if eerr != nil {
				continue
			}
			for _, e := range entries {
				svcRunlevels[e.Name()] = append(svcRunlevels[e.Name()], lvlName)
			}
		}
	}

	started := map[string]bool{}
	if startedEntries, serr := os.ReadDir("/run/openrc/started"); serr == nil {
		for _, e := range startedEntries {
			started[e.Name()] = true
		}
	}

	crashed := map[string]bool{}
	if failedEntries, ferr := os.ReadDir("/run/openrc/failed"); ferr == nil {
		for _, e := range failedEntries {
			crashed[e.Name()] = true
		}
	}

	if initdEntries, ierr := os.ReadDir("/etc/init.d"); ierr == nil {
		for _, e := range initdEntries {
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			if _, known := svcRunlevels[name]; !known {
				svcRunlevels[name] = nil
			}
		}
	}

	result := make([]OpenRCService, 0, len(svcRunlevels))
	for name, runlevels := range svcRunlevels {
		var state ServiceState
		switch {
		case crashed[name]:
			state = ServiceCrashed
		case started[name]:
			state = ServiceStarted
		case len(runlevels) > 0:
			state = ServiceStopped
		default:
			state = ServiceInactive
		}

		sortedLevels := make([]string, len(runlevels))
		copy(sortedLevels, runlevels)
		for i := 0; i < len(sortedLevels); i++ {
			for j := i + 1; j < len(sortedLevels); j++ {
				if sortedLevels[j] < sortedLevels[i] {
					sortedLevels[i], sortedLevels[j] = sortedLevels[j], sortedLevels[i]
				}
			}
		}

		result = append(result, OpenRCService{
			Name:       name,
			State:      state,
			Runlevels:  sortedLevels,
			Supervised: started[name],
		})
	}

	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Name < result[i].Name {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// ── /api/services ────────────────────────────────────────────────────────────

// ServicesHandler retourne l'état des services OpenRC.
// Portage de : rc-model.lua (utilisait /sbin/rc-status + daemoncontrol — binaires)
//
// Query params :
//
//	?state=started   → filtre par état (started|stopped|crashed|inactive)
//	?runlevel=<lvl>  → filtre par runlevel (default|sysinit|boot|nonetwork)
func (b *Broker) ServicesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	services := CollectOpenRCServices()

	if stateFilter := r.URL.Query().Get("state"); stateFilter != "" {
		filtered := services[:0]
		for _, s := range services {
			if string(s.State) == stateFilter {
				filtered = append(filtered, s)
			}
		}
		services = filtered
	}

	if lvlFilter := r.URL.Query().Get("runlevel"); lvlFilter != "" {
		filtered := services[:0]
		for _, s := range services {
			for _, rl := range s.Runlevels {
				if rl == lvlFilter {
					filtered = append(filtered, s)
					break
				}
			}
		}
		services = filtered
	}

	stats := map[string]int{
		"started":  0,
		"stopped":  0,
		"crashed":  0,
		"inactive": 0,
	}
	for _, s := range services {
		stats[string(s.State)]++
	}

	if services == nil {
		services = []OpenRCService{}
	}

	type servicesResponse struct {
		Count    int             `json:"count"`
		Stats    map[string]int  `json:"stats"`
		Services []OpenRCService `json:"services"`
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(servicesResponse{
		Count:    len(services),
		Stats:    stats,
		Services: services,
	})
}

// ============================================================
// LOG FILE READER — /var/log/
// ============================================================

// LogFileInfo décrit un fichier de log disponible.
type LogFileInfo struct {
	Name   string  `json:"name"`    // basename ex. "messages"
	Path   string  `json:"path"`    // chemin complet
	SizeMB float64 `json:"size_mb"` // taille en MiB
	Exists bool    `json:"exists"`
}

// knownLogFiles liste les fichiers de log Alpine standard.
// Liste explicite : KISS + surface d'attaque réduite (pas de traversal arbitraire).
var knownLogFiles = []string{
	"/var/log/messages",
	"/var/log/syslog",
	"/var/log/kern.log",
	"/var/log/auth.log",
	"/var/log/dmesg",
	"/var/log/cron",
	"/var/log/daemon.log",
	"/var/log/lbu.log",
}

// ListLogFiles retourne les fichiers de log disponibles sur ce système.
func ListLogFiles() []LogFileInfo {
	var result []LogFileInfo
	for _, p := range knownLogFiles {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		result = append(result, LogFileInfo{
			Name:   trimPath(p),
			Path:   p,
			SizeMB: float64(info.Size()) / 1024 / 1024,
			Exists: true,
		})
	}
	return result
}

// TailLogFile retourne les N dernières lignes d'un fichier de log.
// Lua utilisait `tail -n N` (binaire). Ici : lecture directe + scan arrière.
//
// Sécurité : le chemin doit être dans /var/log/ ou /run/log/.
// Tout chemin hors de cette liste blanche est refusé (path traversal).
func TailLogFile(path string, n int) ([]string, error) {
	if !isAllowedLogPath(path) {
		return nil, os.ErrPermission
	}
	if n <= 0 || n > 1000 {
		n = 100
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for i, line := range lines {
		line = ansiEscapePattern.ReplaceAllString(line, "")
		line = strings.Map(func(r rune) rune {
			if r < 32 && r != '\t' {
				return -1
			}
			return r
		}, line)
		lines[i] = strings.TrimRight(line, " ")
	}
	return lines, nil
}

func isAllowedLogPath(p string) bool {
	allowed := []string{"/var/log/", "/run/log/", "/var/run/log/"}
	for _, prefix := range allowed {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	for _, known := range knownLogFiles {
		if p == known {
			return true
		}
	}
	return false
}

// ── /api/logs ────────────────────────────────────────────────────────────────

// LogsListHandler retourne la liste des fichiers de log disponibles.
// Portage de : logfiles-model.lua (utilisait posix.files sur /var/log)
// Sécurité : liste blanche explicite — pas de traversal arbitraire.
func (b *Broker) LogsListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	files := ListLogFiles()
	if files == nil {
		files = []LogFileInfo{}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(files)
}

// LogsTailHandler retourne les N dernières lignes d'un fichier de log.
// Portage de : logfiles-model.lua tail + logfiles-tail-html.lsp
//
// Candidat naturel pour le SSE streaming (tail -f live) — prévu comme
// extension : /sse/log?file=messages
//
// Query params :
//
//	?file=<basename>   → nom du fichier (ex. "messages") — OBLIGATOIRE
//	?n=<count>         → nombre de lignes (défaut 100, max 1000)
//
// Sécurité : seuls les fichiers de la whitelist sont accessibles.
func (b *Broker) LogsTailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileName := r.URL.Query().Get("file")
	if fileName == "" {
		http.Error(w, `{"error":"missing ?file= parameter"}`, http.StatusBadRequest)
		return
	}

	for _, c := range fileName {
		if c == '/' || c == '.' || c == '\\' {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"invalid file name"}`))
			return
		}
	}
	filePath := "/var/log/" + fileName

	n := 100
	if nStr := r.URL.Query().Get("n"); nStr != "" {
		if parsed, err := strconv.Atoi(nStr); err == nil {
			n = parsed
		}
	}

	lines, err := TailLogFile(filePath, n)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"error":"file not accessible"}`))
		return
	}

	type tailResponse struct {
		File  string   `json:"file"`
		Lines []string `json:"lines"`
		Count int      `json:"count"`
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(tailResponse{
		File:  fileName,
		Lines: lines,
		Count: len(lines),
	})
}

// ============================================================
// HELPERS internes
// ============================================================

// trim est un alias local pour éviter de répéter strings.TrimSpace.
func trim(s string) string {
	return strings.TrimSpace(s)
}

// trimPath extrait le dernier composant d'un chemin (équivalent filepath.Base
// mais sans import supplémentaire).
func trimPath(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// isCardEntry retourne true uniquement pour "cardN" (chiffre seul après "card").
func isCardEntry(name string) bool {
	if !strings.HasPrefix(name, "card") {
		return false
	}
	suffix := name[4:]
	if suffix == "" {
		return false
	}
	for _, c := range suffix {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// extractPCIFromUevent lit le fichier uevent et extrait PCI_SLOT_NAME.
func extractPCIFromUevent(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PCI_SLOT_NAME=") {
			return strings.TrimPrefix(line, "PCI_SLOT_NAME=")
		}
	}
	return ""
}

// isPCIDisplayClass retourne true si la classe PCI est un contrôleur display.
func isPCIDisplayClass(classHex string) bool {
	if len(classHex) < 4 {
		return false
	}
	s := strings.TrimPrefix(strings.ToLower(classHex), "0x")
	if len(s) < 4 {
		return false
	}
	return strings.HasPrefix(s, "03")
}

// resolveVendor convertit un code vendeur PCI hex en nom lisible.
func resolveVendor(hexCode string) string {
	switch strings.ToLower(strings.TrimPrefix(hexCode, "0x")) {
	case "8086":
		return "Intel Corporation"
	case "1002":
		return "Advanced Micro Devices"
	case "10de":
		return "NVIDIA Corporation"
	case "1234":
		return "QEMU Virtual GPU"
	case "1af4":
		return "VirtIO GPU"
	case "15ad":
		return "VMware SVGA"
	case "1414":
		return "Microsoft Hyper-V GPU"
	}
	if hexCode != "" {
		return "GPU " + hexCode
	}
	return "Unknown"
}

// resolveGPUName construit un nom lisible depuis vendor + device ID + driver.
func resolveGPUName(vendor, deviceHex, driver string) string {
	if strings.Contains(vendor, "Intel") && driver == "i915" {
		return "Intel Graphics (i915)"
	}
	if strings.Contains(vendor, "Intel") && driver == "xe" {
		return "Intel Arc Graphics (xe)"
	}
	if strings.Contains(vendor, "AMD") {
		if driver == "amdgpu" {
			return "AMD Radeon (amdgpu)"
		}
		if driver == "radeon" {
			return "AMD Radeon (radeon)"
		}
	}
	if strings.Contains(vendor, "NVIDIA") {
		if driver == "nouveau" {
			return "NVIDIA (nouveau)"
		}
		if driver != "" {
			return "NVIDIA (" + driver + ")"
		}
	}
	if vendor == "QEMU Virtual GPU" || vendor == "VirtIO GPU" {
		return vendor
	}
	if deviceHex != "" && deviceHex != "0x0000" {
		return vendor + " [" + deviceHex + "]"
	}
	return vendor
}
