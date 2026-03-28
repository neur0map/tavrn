package main

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/term"

	"tavrn.sh/internal/jukebox"
)

var version = "dev"

// Track active mpv process so we can kill it on exit
var (
	activeMPV   *os.Process
	activeMPVMu sync.Mutex
)

const (
	serverAddr = "tavrn.sh:22"
	devAddr    = "localhost:2222"
)

func main() {
	noAudio := false
	dev := false

	for _, arg := range os.Args[1:] {
		switch arg {
		case "--version":
			fmt.Printf("tavrn %s\n", version)
			return
		case "--update":
			if err := runUpdate(); err != nil {
				log.Fatalf("update: %v", err)
			}
			return
		case "--no-audio":
			noAudio = true
		case "--dev":
			dev = true
		case "--help", "-h":
			fmt.Println("Usage:")
			fmt.Println("  tavrn              Connect to tavrn.sh with audio")
			fmt.Println("  tavrn --no-audio   Connect without audio")
			fmt.Println("  tavrn --dev        Connect to localhost:2222")
			fmt.Println("  tavrn --update     Update the local client binary")
			fmt.Println("  tavrn --version    Print version")
			return
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", arg)
			os.Exit(1)
		}
	}

	// Check mpv is available; prompt user if missing.
	if !noAudio {
		if _, err := exec.LookPath("mpv"); err != nil {
			noAudio = promptMissingMPV()
		}
	}

	addr := serverAddr
	if dev {
		addr = devAddr
	}

	connect(addr, noAudio)
}

// promptMissingMPV tells the user mpv is missing, shows the install command
// for their OS, and asks whether to continue without music.
// Returns true if user wants to continue without audio, false to exit.
func promptMissingMPV() bool {
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  mpv is not installed — it's needed for jukebox audio.")
	fmt.Fprintln(os.Stderr, "")

	switch runtime.GOOS {
	case "darwin":
		fmt.Fprintln(os.Stderr, "  Install with Homebrew:")
		fmt.Fprintln(os.Stderr, "    brew install mpv")
	case "linux":
		fmt.Fprintln(os.Stderr, "  Install with your package manager:")
		fmt.Fprintln(os.Stderr, "    Ubuntu/Debian:  sudo apt install mpv")
		fmt.Fprintln(os.Stderr, "    Fedora:         sudo dnf install mpv")
		fmt.Fprintln(os.Stderr, "    Arch:           sudo pacman -S mpv")
	default:
		fmt.Fprintln(os.Stderr, "  Install mpv from https://mpv.io/installation/")
	}

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintf(os.Stderr, "  Continue without music? [Y/n] ")

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer == "n" || answer == "no" {
		fmt.Fprintln(os.Stderr, "  Install mpv and run tavrn again. See you soon!")
		os.Exit(0)
	}

	fmt.Fprintln(os.Stderr, "  Connecting without music... (use --no-audio to skip this prompt)")
	fmt.Fprintln(os.Stderr, "")
	return true
}

func connect(addr string, noAudio bool) {
	authMethods := sshAuthMethods()

	config := &ssh.ClientConfig{
		User:            "tavrn",
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		log.Fatalf("session: %v", err)
	}
	defer session.Close()

	// Raw terminal
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		log.Fatalf("terminal: %v", err)
	}
	defer term.Restore(fd, oldState)

	w, h, _ := term.GetSize(fd)
	if err := session.RequestPty("xterm-256color", h, w, ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		term.Restore(fd, oldState)
		log.Fatalf("pty: %v", err)
	}

	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if !noAudio {
		go startAudioChannel(ctx, conn)
	}

	go handleResize(fd, session)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
		killMPV()
		session.Close()
	}()

	if err := session.Shell(); err != nil {
		term.Restore(fd, oldState)
		log.Fatalf("shell: %v", err)
	}

	session.Wait()
	cancel()
	killMPV()
}

func killMPV() {
	activeMPVMu.Lock()
	p := activeMPV
	activeMPV = nil
	activeMPVMu.Unlock()
	if p != nil {
		// Kill the entire process group
		syscall.Kill(-p.Pid, syscall.SIGKILL)
		p.Kill()
	}
}

// startAudioChannel opens the "tavrn-audio" SSH channel and plays audio via mpv.
func startAudioChannel(ctx context.Context, conn *ssh.Client) {
	ch, reqs, err := conn.OpenChannel("tavrn-audio", nil)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)
	defer ch.Close()

	br := bufio.NewReaderSize(ch, 256*1024)

	for {
		if ctx.Err() != nil {
			return
		}

		_, err := jukebox.DecodeTrackHeader(br)
		if err != nil {
			return
		}

		audioLen, err := jukebox.DecodeAudioLength(br)
		if err != nil {
			return
		}

		playTrack(ctx, br, int64(audioLen))
	}
}

func playTrack(ctx context.Context, r io.Reader, audioLen int64) {
	// Use io.Pipe so we control the flow of data to mpv.
	// This way mpv only has what we've written — killing the pipe stops it.
	pr, pw := io.Pipe()

	cmd := exec.Command("mpv",
		"--no-video",
		"--no-terminal",
		"--demuxer-max-bytes=2MiB",
		"-",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdin = pr
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return
	}

	activeMPVMu.Lock()
	activeMPV = cmd.Process
	activeMPVMu.Unlock()

	// Feed audio data to mpv in chunks, watching for context cancellation
	feedDone := make(chan struct{})
	go func() {
		defer pw.Close()
		defer close(feedDone)

		limited := io.LimitReader(r, audioLen)
		buf := make([]byte, 8192)
		for {
			if ctx.Err() != nil {
				return
			}
			n, err := limited.Read(buf)
			if n > 0 {
				if _, werr := pw.Write(buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Wait for mpv to finish or context to cancel
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-done:
		// Track finished playing
	case <-ctx.Done():
		// Session ended — kill mpv
		pw.Close()
		killMPV()
		<-done
		return
	}

	<-feedDone

	activeMPVMu.Lock()
	activeMPV = nil
	activeMPVMu.Unlock()
}

func handleResize(fd int, session *ssh.Session) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGWINCH)
	for range sigs {
		w, h, err := term.GetSize(fd)
		if err == nil {
			session.WindowChange(h, w)
		}
	}
}

// sshAuthMethods returns auth methods for the connection.
// System SSH keys and agent are tried first so existing tavern identities
// (from plain `ssh tavrn.sh`) are preserved. If no system keys exist, a
// tavrn-specific key is auto-generated at ~/.config/tavrn/id_ed25519.
func sshAuthMethods() []ssh.AuthMethod {
	var signers []ssh.Signer

	// 1. SSH agent.
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			agentClient := agent.NewClient(conn)
			if agentSigners, err := agentClient.Signers(); err == nil {
				signers = append(signers, agentSigners...)
			}
		}
	}

	// 2. Standard key files.
	home, _ := os.UserHomeDir()
	for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
		data, err := os.ReadFile(filepath.Join(home, ".ssh", name))
		if err != nil {
			continue
		}
		key, err := ssh.ParseRawPrivateKey(data)
		if err != nil {
			continue // skip passphrase-protected keys silently
		}
		if signer, err := ssh.NewSignerFromKey(key); err == nil {
			signers = append(signers, signer)
		}
	}

	// 3. Tavrn identity key — auto-generated if nothing above was found.
	if len(signers) == 0 {
		signer, err := ensureTavrnKey()
		if err != nil {
			log.Fatalf("identity key: %v", err)
		}
		signers = append(signers, signer)
	}

	return []ssh.AuthMethod{ssh.PublicKeys(signers...)}
}

// ensureTavrnKey loads or creates a persistent identity key at
// ~/.config/tavrn/id_ed25519. The key is created silently on first run.
func ensureTavrnKey() (ssh.Signer, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	keyPath := filepath.Join(home, ".config", "tavrn", "id_ed25519")

	// Load existing key.
	if data, err := os.ReadFile(keyPath); err == nil {
		if key, err := ssh.ParseRawPrivateKey(data); err == nil {
			if signer, err := ssh.NewSignerFromKey(key); err == nil {
				return signer, nil
			}
		}
	}

	// Generate a fresh ed25519 key.
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	pemBlock, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return nil, fmt.Errorf("marshal key: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(pemBlock), 0600); err != nil {
		return nil, fmt.Errorf("write key: %w", err)
	}

	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, fmt.Errorf("signer: %w", err)
	}
	return signer, nil
}

func runUpdate() error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("go is required to update tavrn via --update")
	}

	fmt.Println("Updating tavrn...")
	cmd := exec.Command("go", "install", "tavrn.sh/cmd/tavrn@latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		return err
	}

	fmt.Println("Client update complete.")
	return nil
}
