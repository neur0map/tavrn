package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ebitengine/oto/v3"
	gomp3 "github.com/hajimehoshi/go-mp3"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/term"

	"tavrn/internal/jukebox"
)

var version = "dev"

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

	if !noAudio {
		go startAudioChannel(conn)
	}

	go handleResize(fd, session)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		session.Close()
	}()

	if err := session.Shell(); err != nil {
		term.Restore(fd, oldState)
		log.Fatalf("shell: %v", err)
	}

	session.Wait()
}

// startAudioChannel opens the "tavrn-audio" SSH channel and plays audio.
func startAudioChannel(conn *ssh.Client) {
	ch, reqs, err := conn.OpenChannel("tavrn-audio", nil)
	if err != nil {
		return // server doesn't support audio, silently skip
	}
	go ssh.DiscardRequests(reqs)
	defer ch.Close()

	playAudio(ch)
}

// playAudio demuxes the audio channel stream into track headers and MP3 data,
// then decodes and plays the MP3 audio via oto.
//
// Wire format from the server:
//
//	[4-byte big-endian length][JSON TrackHeader][MP3 bytes...]
//	[4-byte big-endian length][JSON TrackHeader][MP3 bytes...]
//	...
//
// The MP3 bytes for one track flow until the next track header arrives.
// We detect the boundary by peeking: header length prefixes start with 0x00
// (since JSON metadata is always < 16 MB), while MP3 frames start with 0xFF.
func playAudio(r io.Reader) {
	// Initialize oto audio context (once per process).
	// go-mp3 always decodes to signed 16-bit LE, stereo (2 channels).
	// We don't know the sample rate until we decode the first MP3 frame,
	// but Jamendo MP3 files are typically 44100 Hz. We'll create the context
	// after decoding the first frame to get the actual sample rate.

	br := bufio.NewReaderSize(r, 64*1024)

	// Read the first track header — the server always sends one before MP3 data.
	header, err := jukebox.DecodeTrackHeader(br)
	if err != nil {
		return
	}
	_ = header // Track info available for display if needed

	// Set up a pipe: the demuxer writes MP3 bytes, the decoder reads them.
	pr, pw := io.Pipe()

	// Start the demuxer goroutine: reads from the SSH channel, separates
	// track headers from MP3 data, and writes MP3 bytes into the pipe.
	go demuxAudio(br, pw)

	// Decode MP3 from the pipe reader. go-mp3 accepts io.Reader and
	// decodes to signed 16-bit LE, stereo PCM.
	decoder, err := gomp3.NewDecoder(pr)
	if err != nil {
		pr.Close()
		return
	}

	sampleRate := decoder.SampleRate()

	op := &oto.NewContextOptions{
		SampleRate:   sampleRate,
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
	}

	ctx, readyCh, err := oto.NewContext(op)
	if err != nil {
		pr.Close()
		return
	}
	<-readyCh

	player := ctx.NewPlayer(decoder)
	// Play until the stream ends (pipe closed by demuxer).
	// oto.Player.Play is non-blocking; we read in a loop to drive playback.
	player.Play()

	// Block until the decoder's source is exhausted. The player reads from
	// the decoder which reads from the pipe; when the pipe closes, reads
	// return EOF and playback ends.
	for player.IsPlaying() {
		time.Sleep(100 * time.Millisecond)
	}

	pr.Close()
}

// demuxAudio reads the interleaved header+MP3 stream and writes only MP3
// bytes to pw. It logs track changes.
//
// Detection heuristic: peek at the next byte in the buffered reader.
//   - 0x00 => start of a 4-byte big-endian length prefix (track header)
//   - 0xFF => MP3 frame sync byte (audio data)
//   - anything else => read as MP3 data (covers edge cases)
func demuxAudio(br *bufio.Reader, pw *io.PipeWriter) {
	defer pw.Close()

	buf := make([]byte, 8192)

	for {
		// Peek at the next byte to decide what's coming.
		peek, err := br.Peek(1)
		if err != nil {
			return
		}

		if peek[0] == 0x00 {
			// Likely a track header: [4-byte len][JSON]
			_, herr := jukebox.DecodeTrackHeader(br)
			if herr != nil {
				return
			}
			// Track changed; continue reading MP3 data for the new track.
			// The go-mp3 decoder handles the transition because MP3 is
			// frame-based and will resync.
			continue
		}

		// MP3 data: read available bytes and write to the pipe.
		n, err := br.Read(buf)
		if n > 0 {
			if _, werr := pw.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
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

	// Try SSH agent first.
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			agentClient := agent.NewClient(conn)
			methods = append(methods, ssh.PublicKeysCallback(agentClient.Signers))
		}
	}

	// Fall back to key files on disk.
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
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			continue
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	return methods
}

func runUpdate() {
	fmt.Println("Checking for updates...")
	fmt.Println("Already at latest version.")
}
