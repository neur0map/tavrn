package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"tavrn.sh/internal/hub"
	"tavrn.sh/internal/jukebox"
	"tavrn.sh/internal/server"
	"tavrn.sh/internal/session"
	"tavrn.sh/internal/store"
)

const bannerFile = ".banner"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "purge":
			runPurge()
			return
		case "--message":
			if len(os.Args) < 3 {
				fmt.Println("Usage: tavrn --message \"your message here\"")
				os.Exit(1)
			}
			runMessage(os.Args[2])
			return
		case "--update":
			if err := runUpdate(); err != nil {
				log.Fatalf("update: %v", err)
			}
			return
		case "help", "--help", "-h":
			fmt.Println("Usage:")
			fmt.Println("  tavrn                       Start the SSH server")
			fmt.Println("  tavrn purge                 Purge all data")
			fmt.Println("  tavrn --message \"text\"      Send banner to all connected users")
			fmt.Println("  tavrn --update              Pull main, rebuild, and restart the service")
			return
		}
	}

	runServer()
}

func getPort() int {
	if p := os.Getenv("TAVRN_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			return n
		}
	}
	return 2222
}

func runMessage(text string) {
	if err := os.WriteFile(bannerFile, []byte(text), 0600); err != nil {
		log.Fatalf("failed to write banner: %v", err)
	}
	fmt.Printf("Banner sent: %s\n", text)
}

func runPurge() {
	st, err := store.New("tavrn.db")
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	fmt.Println("Purging all data (users, chat, gallery, visitors)...")
	fmt.Println("Bans will be preserved.")
	if err := st.PurgeAll(); err != nil {
		log.Fatalf("purge failed: %v", err)
	}
	fmt.Println("Done. All data purged.")
}

func runServer() {
	st, err := store.New("tavrn.db")
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	h := hub.New()
	go h.Run()

	if _, err := os.Stat(".ssh"); os.IsNotExist(err) {
		os.MkdirAll(".ssh", 0700)
	}

	var backends []jukebox.MusicBackend
	if jamendoID := os.Getenv("JAMENDO_CLIENT_ID"); jamendoID != "" {
		backends = append(backends, jukebox.NewJamendo(jamendoID))
		log.Printf("Jamendo backend enabled")
	}
	jukeboxEngine := jukebox.NewEngine(backends)
	streamer := jukebox.NewStreamer()
	jukeboxEngine.SetOnTrackChange(func(track jukebox.Track) {
		streamer.StreamTrack(track)
	})

	port := getPort()
	srv, err := server.New(server.Config{
		Host:          "0.0.0.0",
		Port:          port,
		HostKeyPath:   ".ssh/id_ed25519",
		Store:         st,
		Hub:           h,
		JukeboxEngine: jukeboxEngine,
		Streamer:      streamer,
	})
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startPurgeScheduler(st, h)
	go startGalleryCleanup(st, h)
	go watchBannerFile(h)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.Start(ctx); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	log.Printf("tavrn.sh is open. ssh localhost -p %d", port)

	<-done
	log.Println("tavern closing...")
	cancel()
	h.BroadcastAll(session.Msg{
		Type: session.MsgSystem,
		Text: "the tavern is closing...",
	})
	srv.Shutdown(5 * time.Second)
	log.Println("goodbye.")
}

func runUpdate() error {
	if os.Geteuid() == 0 {
		return fmt.Errorf("run tavrn --update as the tavrn user, not root")
	}

	repoDir, err := executableRepoDir()
	if err != nil {
		return err
	}

	env := updateEnv()
	if err := ensureCleanTrackedFiles(repoDir, env); err != nil {
		return err
	}

	fmt.Println("Fetching latest main...")
	if err := runCommand(repoDir, env, "git", "fetch", "origin", "main"); err != nil {
		return err
	}

	fmt.Println("Pulling latest main...")
	if err := runCommand(repoDir, env, "git", "pull", "--ff-only", "origin", "main"); err != nil {
		return err
	}

	fmt.Println("Building tavrn...")
	if err := runCommand(repoDir, env, "go", "build", "-o", "tavrn", "./cmd/tavrn"); err != nil {
		return err
	}

	fmt.Println("Building tavrn-client...")
	if err := runCommand(repoDir, env, "go", "build", "-o", "tavrn-client", "./cmd/tavrn-client"); err != nil {
		return err
	}

	fmt.Println("Finalizing update...")
	if err := runCommand(repoDir, env, "sudo", "/usr/local/sbin/tavrn-finalize-update"); err != nil {
		return err
	}

	rev, err := commandOutput(repoDir, env, "git", "rev-parse", "--short", "HEAD")
	if err != nil {
		return err
	}

	fmt.Printf("Update complete. Running commit %s\n", rev)
	return nil
}

func executableRepoDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe), nil
}

func ensureCleanTrackedFiles(repoDir string, env []string) error {
	out, err := commandOutput(repoDir, env, "git", "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("repo has local tracked changes; commit or revert them before updating")
	}
	return nil
}

func updateEnv() []string {
	pathParts := []string{"/usr/local/go/bin"}
	if current := os.Getenv("PATH"); current != "" {
		pathParts = append(pathParts, current)
	}

	env := os.Environ()
	env = append(env, "PATH="+strings.Join(pathParts, ":"))
	return env
}

func runCommand(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func commandOutput(dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func startPurgeScheduler(st *store.Store, h *hub.Hub) {
	for {
		now := time.Now().UTC()
		daysUntilSunday := (7 - int(now.Weekday())) % 7
		if daysUntilSunday == 0 && (now.Hour() > 23 || (now.Hour() == 23 && now.Minute() >= 59)) {
			daysUntilSunday = 7
		}
		next := time.Date(now.Year(), now.Month(), now.Day()+daysUntilSunday, 23, 59, 0, 0, time.UTC)
		timer := time.NewTimer(time.Until(next))
		<-timer.C

		log.Println("Weekly purge starting...")
		h.BroadcastAll(session.Msg{
			Type: session.MsgSystem,
			Text: "The tavern has been swept clean.",
		})
		st.PurgeAll()
		log.Println("Weekly purge complete")
	}
}

func startGalleryCleanup(st *store.Store, h *hub.Hub) {
	for {
		now := time.Now().UTC()
		next := now.Truncate(time.Hour).Add(time.Hour)
		timer := time.NewTimer(time.Until(next))
		<-timer.C

		log.Println("Gallery cleanup starting...")
		st.ClearGallery()
		h.Broadcast("gallery", session.Msg{
			Type: session.MsgSystem,
			Text: "The gallery board has been wiped clean.",
			Room: "gallery",
		})
		log.Println("Gallery cleanup complete")
	}
}

func watchBannerFile(h *hub.Hub) {
	for {
		time.Sleep(1 * time.Second)

		data, err := os.ReadFile(bannerFile)
		if err != nil {
			continue
		}

		text := strings.TrimSpace(string(data))
		if text == "" {
			continue
		}

		os.Remove(bannerFile)

		log.Printf("Broadcasting banner: %s", text)
		h.BroadcastAll(session.Msg{
			Type: session.MsgBanner,
			Text: text,
		})
	}
}
