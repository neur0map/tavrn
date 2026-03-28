package admin

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildAdmin(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "tavrn-admin")
	cmd := exec.Command("go", "build", "-o", bin, "../../cmd/tavrn-admin")
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

func TestAdminHelp(t *testing.T) {
	bin := buildAdmin(t)
	out, err := exec.Command(bin, "help").CombinedOutput()
	if err != nil {
		t.Fatalf("help failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "Maintainer commands") {
		t.Errorf("expected 'Maintainer commands' in help output, got: %s", output)
	}
	if !strings.Contains(output, "--update") {
		t.Errorf("expected '--update' in help output")
	}
	if !strings.Contains(output, "--message") {
		t.Errorf("expected '--message' in help output")
	}
	if !strings.Contains(output, "purge") {
		t.Errorf("expected 'purge' in help output")
	}
	if !strings.Contains(output, "--add-room") {
		t.Errorf("expected '--add-room' in help output")
	}
}

func TestAdminHelpFlags(t *testing.T) {
	bin := buildAdmin(t)
	for _, flag := range []string{"--help", "-h"} {
		out, err := exec.Command(bin, flag).CombinedOutput()
		if err != nil {
			t.Fatalf("%s failed: %v\n%s", flag, err, out)
		}
		if !strings.Contains(string(out), "Maintainer commands") {
			t.Errorf("%s did not show help", flag)
		}
	}
}

func TestAdminMessageRequiresText(t *testing.T) {
	bin := buildAdmin(t)
	cmd := exec.Command(bin, "--message")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error when --message has no text")
	}
	if !strings.Contains(string(out), "Usage") {
		t.Errorf("expected usage hint, got: %s", out)
	}
}

func TestAdminAddRoomRequiresName(t *testing.T) {
	bin := buildAdmin(t)
	cmd := exec.Command(bin, "--add-room")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error when --add-room has no name")
	}
	if !strings.Contains(string(out), "Usage") {
		t.Errorf("expected usage hint, got: %s", out)
	}
}

func TestAdminAddRoom(t *testing.T) {
	bin := buildAdmin(t)
	dir := filepath.Dir(bin) // signal files land next to the binary
	cmd := exec.Command(bin, "--add-room", "arena")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--add-room failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Room queued") {
		t.Errorf("expected queue confirmation, got: %s", out)
	}
	// Verify .addroom file was written
	data, err := os.ReadFile(filepath.Join(dir, ".addroom"))
	if err != nil {
		t.Fatalf(".addroom file not created: %v", err)
	}
	if string(data) != "arena" {
		t.Errorf("addroom content = %q, want %q", data, "arena")
	}
}

func TestAdminPurge(t *testing.T) {
	bin := buildAdmin(t)
	dir := t.TempDir()
	cmd := exec.Command(bin, "purge")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("purge failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "Purging all data") {
		t.Errorf("expected purge output, got: %s", output)
	}
	if !strings.Contains(output, "Bans and owners preserved") {
		t.Errorf("expected preservation notice, got: %s", output)
	}
}

func TestAdminMessage(t *testing.T) {
	bin := buildAdmin(t)
	dir := filepath.Dir(bin) // signal files land next to the binary
	cmd := exec.Command(bin, "--message", "test broadcast")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--message failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Banner sent") {
		t.Errorf("expected banner confirmation, got: %s", out)
	}
	// Verify banner file was written
	data, err := os.ReadFile(filepath.Join(dir, ".banner"))
	if err != nil {
		t.Fatalf("banner file not created: %v", err)
	}
	if string(data) != "test broadcast" {
		t.Errorf("banner content = %q, want %q", data, "test broadcast")
	}
}

func TestAdminUpdateRefusesRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test must not run as root")
	}
	// We can't test the root check in CI (not root), but we can verify
	// the update command exists and handles missing repo gracefully.
	bin := buildAdmin(t)
	cmd := exec.Command(bin, "--update")
	cmd.Dir = t.TempDir()
	out, _ := cmd.CombinedOutput()
	// Should fail (not in a git repo), but not panic
	output := string(out)
	if strings.Contains(output, "panic") {
		t.Errorf("update panicked: %s", output)
	}
}
