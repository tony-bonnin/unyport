package xenctl

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

type fakeRunner struct {
	name  string
	args  []string
	out   string
	calls int
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls++
	f.name = name
	f.args = append([]string(nil), args...)
	return []byte(f.out), nil
}

func TestParseList(t *testing.T) {
	out := `Name                                        ID   Mem VCPUs      State   Time(s)
Domain-0                                     0  2048     4     r-----    1234.5
app.vm                                       7  1024     2     -b----      12.0
`
	domains := ParseList(out)
	if len(domains) != 2 {
		t.Fatalf("domains len = %d", len(domains))
	}
	if got, want := domains[1].Name, "app.vm"; got != want {
		t.Fatalf("name = %q, want %q", got, want)
	}
	if got, want := domains[1].State, "b"; got != want {
		t.Fatalf("state = %q, want %q", got, want)
	}
}

func TestMigrateBuildsXLArgsWithoutShell(t *testing.T) {
	runner := &fakeRunner{}
	client := &Client{Runner: runner}
	_, err := client.Migrate(context.Background(), "app.vm", "dom0-b", true, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"migrate", "-l", "app.vm", "dom0-b"}
	if !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("args = %#v, want %#v", runner.args, want)
	}
}

func TestRefusesControlDomainAndBadNames(t *testing.T) {
	client := NewClient()
	if _, err := client.Shutdown(context.Background(), "Domain-0", true); err == nil {
		t.Fatal("expected Domain-0 refusal")
	}
	if _, err := client.Shutdown(context.Background(), "bad name;reboot", true); err == nil {
		t.Fatal("expected invalid domain refusal")
	}
}

func TestCreateAllowsOnlyXenConfigPaths(t *testing.T) {
	client := NewClient()
	if _, err := client.Create(context.Background(), "/tmp/vm.cfg", true); err == nil {
		t.Fatal("expected invalid config path")
	}
	if _, err := client.Create(context.Background(), "/etc/xen/configs/app.cfg", true); err != nil {
		t.Fatalf("expected config path to be accepted: %v", err)
	}
}

func TestHandlerSkipsXLWhenHostIsNotDom0(t *testing.T) {
	runner := &fakeRunner{}
	handler := NewHandler(&Client{Runner: runner}, slog.Default(), func() bool { return false })

	req := httptest.NewRequest(http.MethodGet, "/api/xen/domains", nil)
	rec := httptest.NewRecorder()
	handler.Domains(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("domains status = %d, want %d", rec.Code, http.StatusOK)
	}
	if runner.calls != 0 {
		t.Fatalf("domains called xl %d time(s), want 0", runner.calls)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/xen/info", nil)
	rec = httptest.NewRecorder()
	handler.Info(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("info status = %d, want %d", rec.Code, http.StatusOK)
	}
	if runner.calls != 0 {
		t.Fatalf("info called xl %d time(s), want 0", runner.calls)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/xen/domains/app.vm/actions", strings.NewReader(`{"action":"shutdown"}`))
	rec = httptest.NewRecorder()
	handler.DomainAction(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("action status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if runner.calls != 0 {
		t.Fatalf("action called xl %d time(s), want 0", runner.calls)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/xen/domains/create", strings.NewReader(`{"config_path":"/etc/xen/configs/app.cfg"}`))
	rec = httptest.NewRecorder()
	handler.Create(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("create status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if runner.calls != 0 {
		t.Fatalf("create called xl %d time(s), want 0", runner.calls)
	}
}
