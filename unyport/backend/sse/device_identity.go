package sse

import (
	"math"
	"strings"
	"sync"
)

// Identification matérielle des voisins réseau, entièrement embarquée.
//
// Aucune base externe, aucun paquet supplémentaire, aucune sonde active :
// seuls les signaux passifs déjà collectés sont fusionnés par un score
// déterministe (override UNYPORT_DEVICE_TYPES, hostname DHCP/Kea, préfixe OUI
// de la MAC, nom d'interface, éventuelles observations de services fournies
// par l'appelant). Un type n'est proposé que s'il possède une icône dans le
// catalogue existant ; sinon le voisin reste "unknown" et seul le
// constructeur, quand il est certain, est renseigné.

type neighborOUI struct {
	Vendor     string
	DeviceType string // "" signifie : constructeur connu, usage non déductible
}

type deviceSignal struct {
	Source string
	Type   string
	Weight float64
}

type neighborIdentity struct {
	DeviceType string
	Vendor     string
	Confidence float64
	Sources    []string
	Detection  string // source principale, compatible avec detection_source existant
}

// Poids calibrés : l'override manuel est une certitude, un hostname explicite
// reste fort, un préfixe OUI identifie le constructeur mais pas toujours
// l'usage. Une somme de signaux concordants augmente la confiance, un signal
// isolé ne la gonfle pas artificiellement.
const (
	weightOverride       = 1.0
	weightHostname       = 0.65
	weightPublicIP       = 0.5
	weightVirtualMAC     = 0.6
	weightOUI            = 0.55
	weightService        = 0.35
	weightVirtualIface   = 0.2
	neighborCacheLimit   = 4096
	neighborKeySeparator = "\x1f"
)

// virtualOUIs : préfixes des hyperviseurs. Ils portent l'octet local dans
// l'adresse, donc ils sont testés avant la règle "MAC locale = pas de
// constructeur".
var virtualOUIs = map[string]string{
	"00:16:3e": "Xen",
	"00:05:69": "VMware",
	"00:0c:29": "VMware",
	"00:1c:14": "VMware",
	"00:50:56": "VMware",
	"08:00:27": "VirtualBox",
	"52:54:00": "QEMU/KVM",
	"00:15:5d": "Microsoft Hyper-V",
	"00:1c:42": "Parallels",
}

type ouiGroup struct {
	vendor     string
	deviceType string
	prefixes   []string
}

// Base OUI volontairement courte et vérifiable : constructeurs courants sur un
// LAN domestique ou de PME. L'objectif est un indice fiable, pas l'exhaustivité
// de l'IEEE ; un préfixe absent ou ambigu reste sans type.
var embeddedOUIGroups = []ouiGroup{
	{"Raspberry Pi Foundation", "raspberry_pi", []string{
		"b8:27:eb", "dc:a6:32", "e4:5f:01", "2c:cf:67", "d8:3a:dd", "28:cd:c1",
	}},
	{"Espressif", "esp32_board", []string{
		"24:0a:c4", "24:6f:28", "30:ae:a4", "3c:61:05", "3c:71:bf",
		"40:91:51", "44:17:93", "48:27:e2", "4c:11:ae", "54:5a:a6",
		"5c:cf:7f", "60:01:94", "68:c6:3a", "7c:9e:bd", "7c:df:a1",
		"84:0d:8e", "84:cc:a8", "84:f3:eb", "8c:aa:b5", "90:38:0c",
		"94:b5:55", "a0:20:a6", "a4:cf:12", "ac:67:b2", "b4:e6:2d",
		"bc:dd:c2", "c4:4f:33", "cc:50:e3", "d8:a0:1d", "d8:bf:c0",
		"dc:4f:22", "e8:68:e7", "e8:db:84", "ec:fa:bc", "f0:08:d1",
		"f4:cf:a2", "fc:f5:c4",
	}},
	{"Arduino", "arduino_board", []string{"90:a2:da", "60:45:bd", "74:c2:46", "a8:61:0a"}},
	{"Hikvision", "camera", []string{"44:19:b6", "bc:ad:28", "4c:bd:8f", "a4:14:37"}},
	{"Dahua", "camera", []string{"3c:ef:8c", "90:02:a9", "e0:50:8b"}},
	{"Axis Communications", "camera", []string{"00:40:8c"}},
	{"Wyze", "camera", []string{"2c:aa:8e", "7c:78:b2"}},
	{"Sonos", "smart_speaker", []string{"00:0e:58", "5c:aa:fd", "78:28:ca", "94:9f:3e", "b8:e9:37"}},
	{"Signify (Philips Hue)", "smart_bulb", []string{"00:17:88"}},
	{"Roku", "streaming_player", []string{"b0:a7:37", "cc:6d:a0", "dc:3a:5e"}},
	{"Synology", "nas", []string{"00:11:32", "90:09:d0"}},
	{"QNAP", "nas", []string{"24:5e:be", "00:08:9b"}},
	{"Western Digital", "nas", []string{"00:90:a9", "00:14:ee"}},
	{"Brother Industries", "printer", []string{"00:80:77", "30:05:5c"}},
	{"Seiko Epson", "printer", []string{"00:00:48", "38:9d:92"}},
	{"Lexmark", "printer", []string{"00:04:00", "00:21:b7"}},
	{"Zebra Technologies", "printer", []string{"00:07:4d"}},
	{"MikroTik", "router", []string{"4c:5e:0c", "6c:3b:6b", "2c:c8:1b", "74:4d:28", "18:fd:74"}},
	{"Aruba Networks", "accesspoint", []string{"00:0b:86", "24:de:c6"}},
	{"Ruckus Wireless", "accesspoint", []string{"24:c9:a1", "c4:01:7c"}},
	{"Fortinet", "firewall", []string{"00:09:0f", "70:4c:a5"}},
	{"Sophos", "firewall", []string{"00:1a:8c"}},
	{"Supermicro", "server", []string{"00:25:90", "3c:ec:ef", "ac:1f:6b"}},
	{"Tuya Smart", "iot", []string{"18:69:d8", "50:02:91", "68:57:2d"}},
	{"TP-Link", "", []string{
		"00:1d:0f", "14:cc:20", "30:b5:c2", "50:c7:bf", "54:c8:0f",
		"60:32:b1", "a4:2b:b0", "ac:84:c6", "c0:06:c3", "c4:6e:1f",
		"ec:08:6b", "f4:f2:6d",
	}},
	{"Ubiquiti", "", []string{
		"04:18:d6", "24:a4:3c", "44:d9:e7", "68:d7:9a", "74:ac:b9",
		"78:45:58", "80:2a:a8", "b4:fb:e4", "dc:9f:db", "f0:9f:c2",
	}},
	{"Apple", "", []string{
		"00:03:93", "3c:22:fb", "a4:83:e7", "d0:81:7a", "f0:18:98",
	}},
	{"Samsung Electronics", "", []string{
		"00:12:fb", "34:23:ba", "5c:0a:5b", "78:1f:db", "8c:77:12", "e4:7c:f9",
	}},
	{"LG Electronics", "", []string{"00:1c:62", "2c:54:cf", "58:a2:b5", "a8:16:b2"}},
	{"Amazon Technologies", "", []string{"44:65:0d", "68:37:e9", "74:c2:46", "f0:27:2d"}},
	{"Google", "", []string{"3c:5a:b4", "54:60:09", "f4:f5:d8"}},
	{"NETGEAR", "", []string{"20:4e:7f", "a0:40:a0", "00:14:6c"}},
	{"Cisco Systems", "", []string{"00:00:0c", "00:1b:0c", "00:1e:13"}},
	{"Dell", "", []string{"00:14:22", "b8:2a:72", "f8:bc:12"}},
	{"Hewlett Packard", "", []string{"00:1e:0b", "3c:d9:2b", "9c:b6:54"}},
	{"Intel Corporate", "", []string{"00:1b:21", "3c:97:0e", "7c:5c:f8"}},
}

var embeddedOUI = buildOUITable(embeddedOUIGroups)

func buildOUITable(groups []ouiGroup) map[string]neighborOUI {
	table := make(map[string]neighborOUI, len(groups)*8)
	// Deux constructeurs qui revendiquent le même préfixe s'annulent : mieux
	// vaut aucun indice qu'un constructeur faux.
	conflicts := make(map[string]struct{})
	for _, group := range groups {
		for _, prefix := range group.prefixes {
			prefix = strings.ToLower(prefix)
			if existing, exists := table[prefix]; exists {
				if existing.Vendor != group.vendor {
					conflicts[prefix] = struct{}{}
				}
				continue
			}
			table[prefix] = neighborOUI{Vendor: group.vendor, DeviceType: group.deviceType}
		}
	}
	for prefix := range conflicts {
		delete(table, prefix)
	}
	return table
}

// serviceDeviceHints traduit des observations passives de services (mDNS, SSDP,
// ports déjà connus) en type d'appareil. Aucune sonde n'est lancée ici.
var serviceDeviceHints = map[string]string{
	"ipp": "printer", "ipps": "printer", "printer": "printer", "jetdirect": "printer", "9100": "printer",
	"rtsp": "camera", "onvif": "camera", "554": "camera",
	"smb": "nas", "afp": "nas", "nfs": "nas", "445": "nas",
	"airplay": "smart_speaker", "raop": "smart_speaker", "sonos": "smart_speaker",
	"hue": "smart_bulb", "zigbee": "smart_bulb",
	"mqtt": "iot", "coap": "iot",
	"ssh": "server", "rdp": "desktop",
	"googlecast": "streaming_player", "chromecast": "streaming_player",
}

var neighborIdentityCache = struct {
	sync.Mutex
	entries map[string]neighborIdentity
}{entries: make(map[string]neighborIdentity)}

// neighborIdentityFor renvoie l'identification d'un voisin. Le résultat est
// mémorisé par tuple d'entrées : la collecte ARP est répétée à chaque snapshot
// et les mêmes voisins y reviennent.
func neighborIdentityFor(ip, mac, iface, inventory, hostname string, services []string) neighborIdentity {
	key := strings.Join([]string{
		strings.TrimSpace(ip),
		normalizeMAC(mac),
		strings.TrimSpace(iface),
		inventory,
		strings.ToLower(strings.TrimSpace(hostname)),
		strings.ToLower(strings.Join(services, ",")),
	}, neighborKeySeparator)

	neighborIdentityCache.Lock()
	cached, ok := neighborIdentityCache.entries[key]
	neighborIdentityCache.Unlock()
	if ok {
		return cached
	}

	identity := computeNeighborIdentity(ip, mac, iface, inventory, hostname, services)

	neighborIdentityCache.Lock()
	if len(neighborIdentityCache.entries) >= neighborCacheLimit {
		neighborIdentityCache.entries = make(map[string]neighborIdentity, neighborCacheLimit)
	}
	neighborIdentityCache.entries[key] = identity
	neighborIdentityCache.Unlock()
	return identity
}

func computeNeighborIdentity(ip, mac, iface, inventory, hostname string, services []string) neighborIdentity {
	oui := lookupOUI(mac)

	// Un override explicite reste prioritaire sur toute heuristique.
	if deviceType := overrideDeviceTypeFor(ip, inventory); deviceType != "" {
		return neighborIdentity{
			DeviceType: deviceType,
			Vendor:     oui.Vendor,
			Confidence: weightOverride,
			Sources:    []string{"manual"},
			Detection:  "manual",
		}
	}

	var signals []deviceSignal
	add := func(source, deviceType string, weight float64) {
		if !isNetworkDeviceType(deviceType) {
			return
		}
		signals = append(signals, deviceSignal{Source: source, Type: deviceType, Weight: weight})
	}

	hostnameType := deviceTypeForHostname(hostname)
	add("dhcp", hostnameType, weightHostname)
	if isPublicNeighborIP(ip) {
		add("public-ip", "router_wan", weightPublicIP)
	}
	if _, known := virtualOUIs[ouiPrefix(mac)]; known {
		add("mac", "server", weightVirtualMAC)
	}
	add("oui", oui.DeviceType, weightOUI)
	for _, service := range services {
		add("service", serviceDeviceType(service), weightService)
	}
	if isVirtualInterface(iface) {
		add("interface", "server", weightVirtualIface)
	}

	identity := neighborIdentity{
		DeviceType: "unknown",
		Vendor:     oui.Vendor,
		Sources:    []string{"arp"},
		Detection:  "arp",
	}
	if len(signals) == 0 {
		return identity
	}

	// Gagnant = plus forte somme de poids ; égalité tranchée par l'ordre des
	// signaux (hostname, IP publique, MAC virtuelle, OUI, service, interface).
	scores := make(map[string]float64, len(signals))
	var order []string
	for _, signal := range signals {
		if _, seen := scores[signal.Type]; !seen {
			order = append(order, signal.Type)
		}
		scores[signal.Type] += signal.Weight
	}
	winner := order[0]
	for _, candidate := range order[1:] {
		if scores[candidate] > scores[winner] {
			winner = candidate
		}
	}
	// A rôle explicite dans le hostname est plus précis que les indices
	// génériques "server" d'une MAC d'hyperviseur ou d'une interface virtuelle.
	if hostnameType != "unknown" && hostnameType != "server" && winner == "server" {
		winner = hostnameType
	}

	confidence := math.Min(1, scores[winner])
	var sources []string
	for _, signal := range signals {
		if signal.Type != winner {
			continue
		}
		if !containsString(sources, signal.Source) {
			sources = append(sources, signal.Source)
		}
	}

	identity.DeviceType = winner
	identity.Confidence = math.Round(confidence*100) / 100
	identity.Sources = sources
	identity.Detection = sources[0]
	return identity
}

// overrideDeviceTypeFor lit l'inventaire "IP=type" (virgule, point-virgule,
// espace ou retour ligne) et applique les mêmes règles de validité que
// l'API existante : types stockage refusés sur le réseau, sauf nas/san.
func overrideDeviceTypeFor(ip, inventory string) string {
	for _, entry := range strings.FieldsFunc(inventory, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
	}) {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) != ip {
			continue
		}
		if deviceType := strings.ToLower(strings.TrimSpace(parts[1])); isNetworkDeviceType(deviceType) {
			return deviceType
		}
	}
	return ""
}

// isNetworkDeviceType n'accepte qu'un type présent dans le catalogue d'icônes
// et pertinent pour un voisin réseau.
func isNetworkDeviceType(deviceType string) bool {
	deviceType = strings.ToLower(strings.TrimSpace(deviceType))
	if deviceType == "" || deviceType == "unknown" || deviceType == "storage" {
		return false
	}
	if validDeviceTypes[deviceType] {
		return true
	}
	networkStorage := deviceType == "nas" || deviceType == "san"
	return validExtendedDeviceTypes[deviceType] && (!validStorageDeviceTypes[deviceType] || networkStorage)
}

func serviceDeviceType(service string) string {
	name := strings.ToLower(strings.TrimSpace(service))
	if deviceType, ok := serviceDeviceHints[name]; ok {
		return deviceType
	}
	return ""
}

func isVirtualInterface(iface string) bool {
	iface = strings.ToLower(strings.TrimSpace(iface))
	return strings.HasPrefix(iface, "vif") ||
		strings.HasPrefix(iface, "tap") ||
		strings.HasPrefix(iface, "vnet")
}

// ouiPrefix renvoie les 3 premiers octets d'une MAC valide, sinon "".
func ouiPrefix(mac string) string {
	normalized := normalizeMAC(mac)
	groups := strings.Split(normalized, ":")
	if len(groups) != 6 {
		return ""
	}
	for _, group := range groups {
		if len(group) != 2 {
			return ""
		}
		for _, r := range group {
			isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
			if !isHex {
				return ""
			}
		}
	}
	return strings.Join(groups[:3], ":")
}

// lookupOUI ne prétend jamais identifier un constructeur pour une MAC locale,
// multicast ou invalide : seul un préfixe global ou un hyperviseur connu donne
// un vendor.
func lookupOUI(mac string) neighborOUI {
	prefix := ouiPrefix(mac)
	if prefix == "" {
		return neighborOUI{}
	}
	if vendor, ok := virtualOUIs[prefix]; ok {
		return neighborOUI{Vendor: vendor, DeviceType: "server"}
	}
	firstByte := hexByte(prefix[0])<<4 | hexByte(prefix[1])
	if firstByte&0x01 != 0 || firstByte&0x02 != 0 {
		return neighborOUI{}
	}
	return embeddedOUI[prefix]
}

func hexByte(r byte) byte {
	switch {
	case r >= '0' && r <= '9':
		return r - '0'
	case r >= 'a' && r <= 'f':
		return r - 'a' + 10
	default:
		return 0
	}
}
