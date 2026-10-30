package sse

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVersionsHandlerDegradesWithoutGatewayError(t *testing.T) {
	b := &Broker{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		versionsFetcher: func(*http.Client, string) (string, string, error) {
			return "", "", errors.New("network unavailable")
		},
	}
	recorder := httptest.NewRecorder()
	b.VersionsHandler(recorder, httptest.NewRequest(http.MethodGet, "/api/versions", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var body struct {
		Available bool `json:"available"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Available {
		t.Fatal("unavailable version source reported as available")
	}
}

func TestVersionsHandlerCachesSuccessfulLookup(t *testing.T) {
	calls := 0
	b := &Broker{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		versionsFetcher: func(*http.Client, string) (string, string, error) {
			calls++
			return "6.18.33-lts", "3.23.4", nil
		},
	}
	for range 2 {
		recorder := httptest.NewRecorder()
		b.VersionsHandler(recorder, httptest.NewRequest(http.MethodGet, "/api/versions", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
	}
	if calls != 1 {
		t.Fatalf("fetch calls = %d, want 1", calls)
	}
}
