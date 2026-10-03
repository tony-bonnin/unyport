package xenctl

import (
	"context"
	"reflect"
	"testing"
)

type fakeRunner struct {
	name string
	args []string
	out  string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
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
