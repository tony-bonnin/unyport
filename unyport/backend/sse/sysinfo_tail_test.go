package sse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTailLogFileReturnsLastLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.log")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lines, err := tailLogFile(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(lines, ",") != "two,three" {
		t.Fatalf("unexpected lines: %q", lines)
	}
}

func TestTailLogFileBoundsLargeInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.log")
	content := strings.Repeat("discarded line\n", 100000) + "kept\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	lines, err := tailLogFile(path, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != "kept" {
		t.Fatalf("unexpected lines: %q", lines)
	}
}
