package sse

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	pollInterval  = 2 * time.Second
	ringSize      = 60 // 2 minutes d'historique en mémoire
)

// Broker collecte les métriques et diffuse vers N clients SSE connectés.
type Broker struct {
	mu       sync.RWMutex
	ring     [ringSize]Snapshot
	head     int
	count    int
	clients  map[chan Snapshot]struct{}
	logger   *slog.Logger
}

func NewBroker(logger *slog.Logger) *Broker {
	b := &Broker{
		clients: make(map[chan Snapshot]struct{}),
		logger:  logger,
	}
	go b.loop()
	return b
}

func (b *Broker) loop() {
	for {
		snap := collect()
		b.mu.Lock()
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

// Handler est le endpoint SSE — nécessite d'être protégé par AuthMiddleware en amont.
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

func writeSSE(w http.ResponseWriter, f http.Flusher, snap Snapshot) {
	data, err := json.Marshal(snap)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	f.Flush()
}

// SystemInfo contient les infos statiques HW/OS — lues une fois au boot.
type SystemInfo struct {
	OS         string `json:"os"`
	OSRelease  string `json:"os_release"`
	OSVersion  string `json:"os_version"`
	Kernel     string `json:"kernel"`
	Date       string `json:"date"`
	Uptime     string `json:"uptime"`
	CPUModel   string `json:"cpu_model"`
	CPUVendor  string `json:"cpu_vendor"`
	CPUCores   int    `json:"cpu_cores"`
	BoardName  string `json:"board_name"`
	BoardVendor string `json:"board_vendor"`
	// Métriques courantes (pour la compatibilité avec l'ancien front)
	CPUUsage    float64 `json:"cpu_usage"`
	CPUFreqAvg  int     `json:"cpu_freq_avg_mhz"`
	CPUFreqMax  int     `json:"cpu_freq_max_mhz"`
	MemTotal    uint64  `json:"mem_total"`
	MemUsed     uint64  `json:"mem_used"`
	MemFree     uint64  `json:"mem_free"`
}

// SystemInfoHandler retourne un snapshot complet (statique + live) en JSON.
func (b *Broker) SystemInfoHandler(w http.ResponseWriter, r *http.Request) {
	model, vendor, cores := getCPUInfo()
	pretty, version := getOSRelease()
	kernel := readFirstFile("/proc/sys/kernel/osrelease")
	uptime := readFirstFile("/proc/uptime")
	board  := readFirstFile("/sys/devices/virtual/dmi/id/board_name")
	bvendor := readFirstFile("/sys/devices/virtual/dmi/id/board_vendor")

	b.mu.RLock()
	var snap Snapshot
	if b.count > 0 {
		snap = b.ring[(b.head-1+ringSize)%ringSize]
	}
	b.mu.RUnlock()

	info := SystemInfo{
		OSRelease:   pretty,
		OSVersion:   version,
		Kernel:      strings.TrimSpace(kernel),
		Date:        time.Now().Format(time.RFC1123),
		Uptime:      strings.TrimSpace(uptime),
		CPUModel:    model,
		CPUVendor:   vendor,
		CPUCores:    cores,
		BoardName:   strings.TrimSpace(board),
		BoardVendor: strings.TrimSpace(bvendor),
		CPUUsage:    snap.CPUUsage,
		CPUFreqAvg:  snap.CPUFreqAvg,
		CPUFreqMax:  snap.CPUFreqMax,
		MemTotal:    snap.MemTotal,
		MemUsed:     snap.MemUsed,
		MemFree:     snap.MemFree,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(info)
}