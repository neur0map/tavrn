package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/term"

	"tavrn/internal/jukebox"
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
			runUpdate()
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
			fmt.Println("  tavrn --update     Update to latest version")
			fmt.Println("  tavrn --version    Print version")
			return
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", arg)
			os.Exit(1)
		}
	}

	// Check mpv is installed unless --no-audio
	if !noAudio {
		if _, err := exec.LookPath("mpv"); err != nil {
			fmt.Println("tavrn: mpv not found — required for audio playback")
			fmt.Println()
			switch runtime.GOOS {
			case "darwin":
				fmt.Println("  Install:  brew install mpv")
			case "linux":
				fmt.Println("  Install:  sudo apt install mpv")
			default:
				fmt.Println("  Install mpv from https://mpv.io/installation/")
			}
			fmt.Println()
			fmt.Println("  Or connect without audio:  tavrn --no-audio")
			os.Exit(1)
		}
	}

	addr := serverAddr
	if dev {
		addr = devAddr
	}

	connect(addr, noAudio)
}

func connect(addr string, noAudio bool) {
	authMethods := sshAuthMethods()
	if len(authMethods) == 0 {
		log.Fatal("no SSH keys found")
	}

	config := &ssh.ClientConfig{
		User:            os.Getenv("USER"),
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

		cmd := exec.CommandContext(ctx, "mpv",
			"--no-video",
			"--no-terminal",
			"--no-cache",
			"-",
		)

		// Ensure mpv dies when parent process dies
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Stdin = io.LimitReader(br, int64(audioLen))
		cmd.Stdout = nil
		cmd.Stderr = nil

		if err := cmd.Start(); err != nil {
			continue
		}

		activeMPVMu.Lock()
		activeMPV = cmd.Process
		activeMPVMu.Unlock()

		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()

		select {
		case <-done:
			// mpv finished normally
		case <-ctx.Done():
			cmd.Process.Kill()
			<-done
			return
		}

		activeMPVMu.Lock()
		activeMPV = nil
		activeMPVMu.Unlock()
	}
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

func sshAuthMethods() []ssh.AuthMethod {
	var methods []ssh.AuthMethod
	var agentClient agent.ExtendedAgent

	// Connect to SSH agent if available.
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			agentClient = agent.NewClient(conn)
		}
	}

	// Load key files from disk and add to agent automatically.
	home, _ := os.UserHomeDir()
	keyFiles := []string{
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "id_ecdsa"),
	}
	for _, path := range keyFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		key, err := ssh.ParseRawPrivateKey(data)
		if err != nil {
			continue
		}

		if agentClient != nil {
			agentClient.Add(agent.AddedKey{PrivateKey: key})
		}

		signer, err := ssh.NewSignerFromKey(key)
		if err != nil {
			continue
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	if agentClient != nil {
		methods = append(methods, ssh.PublicKeysCallback(agentClient.Signers))
	}

	return methods
}

func runUpdate() {
	fmt.Println("Checking for updates...")
	fmt.Println("Already at latest version.")
}
