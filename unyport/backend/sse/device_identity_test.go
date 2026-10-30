package sse

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// resetNeighborIdentityCache isole chaque test du cache partagé.
func resetNeighborIdentityCache(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		neighborIdentityCache.Lock()
		neighborIdentityCache.entries = make(map[string]neighborIdentity)
		neighborIdentityCache.Unlock()
	})
}

func assertSources(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("sources = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sources = %v, want %v (ordre déterministe)", got, want)
		}
	}
}

func TestNeighborIdentityOverrideWins(t *testing.T) {
	resetNeighborIdentityCache(t)
	// Hostname "camera" + MAC Xen + interface virtuelle : l'inventaire manuel
	// doit rester prioritaire sur tous les autres signaux.
	identity := neighborIdentityFor(
		"192.168.2.10", "00:16:3e:aa:bb:cc", "vif12.0",
		"192.168.2.10=smart_tv,192.168.2.11=camera", "camera-jardin", nil,
	)
	if identity.DeviceType != "smart_tv" {
		t.Fatalf("DeviceType = %q, want smart_tv", identity.DeviceType)
	}
	if identity.Detection != "manual" || identity.Confidence != 1 {
		t.Fatalf("Detection/Confidence = %q/%v, want manual/1", identity.Detection, identity.Confidence)
	}
	assertSources(t, identity.Sources, "manual")
}

func TestNeighborIdentityOverrideRejectsNetworkStorage(t *testing.T) {
	resetNeighborIdentityCache(t)
	tests := []struct {
		inventory string
		want      string
	}{
		{"192.168.2.20=storage", "unknown"}, // stockage local : jamais un voisin réseau
		{"192.168.2.20=nas", "nas"},         // stockage réseau : accepté
		{"192.168.2.20=inconnu", "unknown"},
		{"192.168.2.21=server", "unknown"}, // autre IP : override non appliqué
	}
	for _, tt := range tests {
		identity := neighborIdentityFor("192.168.2.20", "aa:bb:cc:00:11:22", "eth0", tt.inventory, "", nil)
		if identity.DeviceType != tt.want {
			t.Fatalf("inventaire %q: DeviceType = %q, want %q", tt.inventory, identity.DeviceType, tt.want)
		}
	}
}

func TestNeighborIdentityVendorWithoutDeviceType(t *testing.T) {
	resetNeighborIdentityCache(t)
	// Préfixe Apple : constructeur certain, usage non déductible.
	identity := neighborIdentityFor("192.168.2.30", "00:03:93:12:34:56", "br-lan", "", "", nil)
	if identity.Vendor != "Apple" {
		t.Fatalf("Vendor = %q, want Apple", identity.Vendor)
	}
	if identity.DeviceType != "unknown" || identity.Detection != "arp" || identity.Confidence != 0 {
		t.Fatalf("identité = %+v, want unknown/arp/0", identity)
	}
	assertSources(t, identity.Sources, "arp")
}

func TestNeighborIdentityHostnameBeatsOUI(t *testing.T) {
	resetNeighborIdentityCache(t)
	// MAC Hikvision (camera) mais hostname explicite de téléviseur : le
	// hostname local reste plus spécifique que le constructeur.
	identity := neighborIdentityFor("192.168.2.40", "44:19:b6:00:00:01", "br-lan", "", "salon-tv", nil)
	if identity.DeviceType != "tv" {
		t.Fatalf("DeviceType = %q, want tv", identity.DeviceType)
	}
	if identity.Vendor != "Hikvision" {
		t.Fatalf("Vendor = %q, want Hikvision", identity.Vendor)
	}
	assertSources(t, identity.Sources, "dhcp")
	if identity.Confidence != weightHostname {
		t.Fatalf("Confidence = %v, want %v", identity.Confidence, weightHostname)
	}
}

func TestNeighborIdentityRecognizesReverseProxyHostname(t *testing.T) {
	resetNeighborIdentityCache(t)
	identity := neighborIdentityFor("192.168.3.2", "00:16:3e:00:03:02", "br-srv", "", "REVERSE-PROXY", nil)
	if identity.DeviceType != "reverse_proxy" {
		t.Fatalf("DeviceType = %q, want reverse_proxy", identity.DeviceType)
	}
	if identity.Detection != "dhcp" {
		t.Fatalf("Detection = %q, want dhcp", identity.Detection)
	}
}

func TestNeighborIdentityAgreementRaisesConfidence(t *testing.T) {
	resetNeighborIdentityCache(t)
	// Hostname Brother + OUI Brother : signaux concordants, confiance plafonnée.
	identity := neighborIdentityFor("192.168.2.50", "00:80:77:00:00:01", "br-lan", "", "brother-printer", nil)
	if identity.DeviceType != "printer" {
		t.Fatalf("DeviceType = %q, want printer", identity.DeviceType)
	}
	assertSources(t, identity.Sources, "dhcp", "oui")
	if identity.Confidence != 1 {
		t.Fatalf("Confidence = %v, want 1 (plafond)", identity.Confidence)
	}
	if identity.Detection != "dhcp" {
		t.Fatalf("Detection = %q, want dhcp (premier signal gagnant)", identity.Detection)
	}
}

func TestNeighborIdentityVirtualMACAndInterface(t *testing.T) {
	resetNeighborIdentityCache(t)
	identity := neighborIdentityFor("192.168.2.60", "00:16:3e:01:02:03", "vif3.0", "", "", nil)
	if identity.DeviceType != "server" {
		t.Fatalf("DeviceType = %q, want server", identity.DeviceType)
	}
	if identity.Vendor != "Xen" {
		t.Fatalf("Vendor = %q, want Xen", identity.Vendor)
	}
	// MAC Xen + interface virtuelle + OUI hyperviseur : trois signaux
	// concordants vers "server", confiance plafonnée à 1.
	assertSources(t, identity.Sources, "mac", "oui", "interface")
	if identity.Confidence != 1 {
		t.Fatalf("Confidence = %v, want 1 (plafond)", identity.Confidence)
	}
}

func TestNeighborIdentityServiceHint(t *testing.T) {
	resetNeighborIdentityCache(t)
	identity := neighborIdentityFor("192.168.2.70", "aa:bb:cc:dd:ee:70", "eth0", "", "", []string{"onvif"})
	if identity.DeviceType != "camera" || identity.Detection != "service" {
		t.Fatalf("identité = %+v, want camera/service", identity)
	}
	assertSources(t, identity.Sources, "service")
}

func TestNeighborIdentityPublicIP(t *testing.T) {
	resetNeighborIdentityCache(t)
	identity := neighborIdentityFor("1.1.1.1", "", "eth0", "", "", nil)
	if identity.DeviceType != "router_wan" || identity.Detection != "public-ip" {
		t.Fatalf("identité = %+v, want router_wan/public-ip", identity)
	}
	if identity.Vendor != "" {
		t.Fatalf("Vendor = %q, want vide sans MAC", identity.Vendor)
	}
}

func TestLookupOUIIgnoresLocalAndInvalidMACs(t *testing.T) {
	tests := []struct {
		mac    string
		vendor string
	}{
		{"02:11:22:33:44:55", ""}, // adresse locale : aucun constructeur certifiable
		{"01:00:5e:00:00:01", ""}, // multicast
		{"zz:bb:cc:dd:ee:ff", ""}, // invalide
		{"44:19:b6", ""},          // tronquée
		{"44:19:b6:AA:BB:CC", "Hikvision"},
		{"00:16:3e:aa:bb:cc", "Xen"}, // préfixe hyperviseur prioritaire
	}
	for _, tt := range tests {
		if got := lookupOUI(tt.mac).Vendor; got != tt.vendor {
			t.Fatalf("lookupOUI(%q).Vendor = %q, want %q", tt.mac, got, tt.vendor)
		}
	}
}

func TestBuildOUITableDropsConflictingPrefixes(t *testing.T) {
	table := buildOUITable([]ouiGroup{
		{"Constructeur A", "printer", []string{"de:ad:be"}},
		{"Constructeur A", "", []string{"de:ad:be"}}, // même constructeur : pas de conflit
		{"Constructeur B", "camera", []string{"de:ad:be"}},
	})
	if entry, ok := table["de:ad:be"]; ok {
		t.Fatalf("préfixe ambigu conservé : %+v", entry)
	}
}

func TestNeighborIdentityIsCached(t *testing.T) {
	resetNeighborIdentityCache(t)
	const ip = "192.168.9.9"
	first := neighborIdentityFor(ip, "00:11:32:00:00:01", "eth0", "", "nas-maison", nil)
	second := neighborIdentityFor(ip, "00:11:32:00:00:01", "eth0", "", "nas-maison", nil)
	if first.DeviceType != second.DeviceType || first.Confidence != second.Confidence {
		t.Fatalf("résultat non stable : %+v vs %+v", first, second)
	}
	neighborIdentityCache.Lock()
	size := len(neighborIdentityCache.entries)
	neighborIdentityCache.Unlock()
	if size != 1 {
		t.Fatalf("cache = %d entrées, want 1", size)
	}
}

func TestNeighborIdentityCacheStaysBounded(t *testing.T) {
	resetNeighborIdentityCache(t)
	for i := 0; i < neighborCacheLimit+64; i++ {
		ip := "10.42." + strconv.Itoa(i/256) + "." + strconv.Itoa(i%256)
		neighborIdentityFor(ip, "", "eth0", "", "", nil)
	}
	neighborIdentityCache.Lock()
	size := len(neighborIdentityCache.entries)
	neighborIdentityCache.Unlock()
	if size > neighborCacheLimit {
		t.Fatalf("cache = %d entrées, want <= %d", size, neighborCacheLimit)
	}
}

// L'enrichissement doit rester additif : aucune clé JSON vide ne doit
// apparaître pour préserver les consommateurs existants de l'API.
func TestARPEntryJSONOmitsEmptyEnrichment(t *testing.T) {
	payload, err := json.Marshal(ARPEntry{IP: "192.168.2.1", MAC: "aa:bb:cc:dd:ee:ff", DeviceType: "unknown"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{`"vendor"`, `"confidence"`, `"sources"`} {
		if strings.Contains(string(payload), key) {
			t.Fatalf("%s présent alors que vide : %s", key, payload)
		}
	}
}
