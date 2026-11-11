package sse

import (
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBrokerCloseReleasesSSEHandler(t *testing.T) {
	b := &Broker{
		clients: make(map[chan Snapshot]struct{}),
		logger:  slog.Default(),
		done:    make(chan struct{}),
	}
	req := httptest.NewRequest("GET", "/sse/system", nil)
	rec := httptest.NewRecorder()
	finished := make(chan struct{})

	go func() {
		b.Handler(rec, req)
		close(finished)
	}()

	deadline := time.After(time.Second)
	for {
		b.mu.RLock()
		connected := len(b.clients) == 1
		b.mu.RUnlock()
		if connected {
			break
		}
		select {
		case <-deadline:
			t.Fatal("SSE handler did not subscribe")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	b.Close()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("SSE handler remained blocked after broker close")
	}
}
