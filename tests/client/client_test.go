package client

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildClient(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "tavrn")
	cmd := exec.Command("go", "build", "-o", bin, "../../cmd/tavrn")
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

func TestClientHelp(t *testing.T) {
	bin := buildClient(t)
	out, err := exec.Command(bin, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("--help failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "Usage") {
		t.Errorf("expected 'Usage' in help output, got: %s", output)
	}
	if !strings.Contains(output, "--no-audio") {
		t.Errorf("expected '--no-audio' in help output")
	}
	if !strings.Contains(output, "--dev") {
		t.Errorf("expected '--dev' in help output")
	}
	if !strings.Contains(output, "--update") {
		t.Errorf("expected '--update' in help output")
	}
	if !strings.Contains(output, "--version") {
		t.Errorf("expected '--version' in help output")
	}
}

func TestClientHelpShort(t *testing.T) {
	bin := buildClient(t)
	out, err := exec.Command(bin, "-h").CombinedOutput()
	if err != nil {
		t.Fatalf("-h failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Usage") {
		t.Errorf("-h did not show usage")
	}
}

func TestClientVersion(t *testing.T) {
	bin := buildClient(t)
	out, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "tavrn") {
		t.Errorf("expected 'tavrn' in version output, got: %s", output)
	}
}

func TestClientVersionContainsDev(t *testing.T) {
	// Default build (no ldflags) should show "dev"
	bin := buildClient(t)
	out, _ := exec.Command(bin, "--version").CombinedOutput()
	if !strings.Contains(string(out), "dev") {
		t.Errorf("expected 'dev' in default version, got: %s", out)
	}
}

func TestClientVersionWithLdflags(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "tavrn")
	cmd := exec.Command("go", "build",
		"-ldflags", "-X main.version=v1.2.3",
		"-o", bin, "../../cmd/tavrn")
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build with ldflags failed: %v\n%s", err, out)
	}
	out, _ = exec.Command(bin, "--version").CombinedOutput()
	if !strings.Contains(string(out), "v1.2.3") {
		t.Errorf("expected 'v1.2.3' in version, got: %s", out)
	}
}

func TestClientUnknownFlag(t *testing.T) {
	bin := buildClient(t)
	cmd := exec.Command(bin, "--bogus")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
	if !strings.Contains(string(out), "unknown flag") {
		t.Errorf("expected 'unknown flag' in output, got: %s", out)
	}
}

func TestClientUpdateRequiresGo(t *testing.T) {
	bin := buildClient(t)
	// Run with a PATH that excludes go
	cmd := exec.Command(bin, "--update")
	cmd.Env = []string{"PATH=/nonexistent", "HOME=" + t.TempDir()}
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error when go is not in PATH")
	}
	if !strings.Contains(string(out), "go is required") {
		t.Errorf("expected 'go is required' error, got: %s", out)
	}
}
