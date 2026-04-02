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
	"text/template"
	"time"

	"tavrn.sh/internal/bartender"
	"tavrn.sh/internal/config"
	"tavrn.sh/internal/gif"
	"tavrn.sh/internal/hub"
	"tavrn.sh/internal/jukebox"
	"tavrn.sh/internal/poll"
	"tavrn.sh/internal/sanitize"
	"tavrn.sh/internal/search"
	"tavrn.sh/internal/server"
	"tavrn.sh/internal/session"
	"tavrn.sh/internal/store"
	"tavrn.sh/internal/sudoku"
	"tavrn.sh/internal/wargame"
	"tavrn.sh/internal/webstream"
)

const bannerFile = ".banner"
const bartenderToggleFile = ".bartender-toggle"
const addRoomFile = ".addroom"
const renameRoomFile = ".renameroom"
const removeRoomFile = ".removeroom"
const banFile = ".ban"
const purgeFile = ".purge"

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
		case "--add-room":
			if len(os.Args) < 3 {
				fmt.Println("Usage: tavrn --add-room \"room_name\"")
				os.Exit(1)
			}
			runAddRoom(os.Args[2])
			return
		case "--rename-room":
			if len(os.Args) < 4 {
				fmt.Println("Usage: tavrn --rename-room \"old_name\" \"new_name\"")
				os.Exit(1)
			}
			runRenameRoom(os.Args[2], os.Args[3])
			return
		case "--remove-room":
			if len(os.Args) < 3 {
				fmt.Println("Usage: tavrn --remove-room \"room_name\"")
				os.Exit(1)
			}
			runRemoveRoom(os.Args[2])
			return
		case "--ban":
			if len(os.Args) < 3 {
				fmt.Println("Usage: tavrn --ban \"nickname\"")
				os.Exit(1)
			}
			runBan(os.Args[2])
			return
		case "--unban":
			if len(os.Args) < 3 {
				fmt.Println("Usage: tavrn --unban \"nickname\"")
				os.Exit(1)
			}
			runUnban(os.Args[2])
			return
		case "--ban-list":
			runBanList()
			return
		case "--clear-banner":
			runClearBanner()
			return
		case "--set-flag":
			if len(os.Args) < 5 {
				fmt.Println("Usage: tavrn --set-flag <wargame> <level> <flag>")
				os.Exit(1)
			}
			runSetFlag(os.Args[2], os.Args[3], os.Args[4])
			return
		case "--list-flags":
			if len(os.Args) < 3 {
				fmt.Println("Usage: tavrn --list-flags <wargame>")
				os.Exit(1)
			}
			runListFlags(os.Args[2])
			return
		case "--bartender-off":
			os.WriteFile(resolvedPath(bartenderToggleFile), []byte("off"), 0600)
			fmt.Println("Bartender disable signal sent.")
			return
		case "--bartender-on":
			os.WriteFile(resolvedPath(bartenderToggleFile), []byte("on"), 0600)
			fmt.Println("Bartender enable signal sent.")
			return
		case "--update":
			if err := runUpdate(); err != nil {
				log.Fatalf("update: %v", err)
			}
			return
		case "help", "--help", "-h":
			fmt.Println("Maintainer commands:")
			fmt.Println("  tavrn                            Start the SSH server")
			fmt.Println("  tavrn purge                      Purge all data")
			fmt.Println("  tavrn --message \"text\"           Send banner to all connected users")
			fmt.Println("  tavrn --clear-banner             Clear the active banner")
			fmt.Println("  tavrn --add-room \"name\"          Add a new room (live, no restart)")
			fmt.Println("  tavrn --rename-room \"old\" \"new\"  Rename a room (live)")
			fmt.Println("  tavrn --remove-room \"name\"       Remove a room (live, moves users to #lounge)")
			fmt.Println("  tavrn --ban \"nickname\"           Ban a user by nickname (live, kicks them)")
			fmt.Println("  tavrn --unban \"nickname\"         Unban a user by nickname")
			fmt.Println("  tavrn --ban-list                 Show all active bans")
			fmt.Println("  tavrn --bartender-off            Disable bartender (live)")
			fmt.Println("  tavrn --bartender-on             Enable bartender (live)")
			fmt.Println("  tavrn --set-flag bandit 1 \"flag\" Set a wargame flag (bandit/natas/leviathan)")
			fmt.Println("  tavrn --list-flags bandit         List levels with flags for a wargame")
			fmt.Println("  tavrn --update                   Pull main, rebuild, restart service")
			fmt.Println("  tavrn --web-audio                Start with web audio streaming on :8090")
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

func hasFlag(flag string) bool {
	for _, arg := range os.Args[1:] {
		if arg == flag {
			return true
		}
	}
	return false
}

func runMessage(text string) {
	if err := os.WriteFile(resolvedPath(bannerFile), []byte(text), 0600); err != nil {
		log.Fatalf("failed to write banner: %v", err)
	}
	fmt.Printf("Banner sent: %s\n", text)
}

func runClearBanner() {
	st, err := store.New(resolvedDBPath())
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()
	st.ClearBanner()
	fmt.Println("Banner cleared.")
}

func runAddRoom(name string) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		fmt.Println("Room name cannot be empty.")
		os.Exit(1)
	}
	if err := os.WriteFile(resolvedPath(addRoomFile), []byte(name), 0600); err != nil {
		log.Fatalf("failed to write addroom file: %v", err)
	}
	fmt.Printf("Room queued: #%s (will appear when server picks it up)\n", name)
}

func runRenameRoom(oldName, newName string) {
	oldName = strings.ToLower(strings.TrimSpace(oldName))
	newName = strings.ToLower(strings.TrimSpace(newName))
	if oldName == "" || newName == "" {
		fmt.Println("Room names cannot be empty.")
		os.Exit(1)
	}
	// Format: "old:new"
	payload := oldName + ":" + newName
	if err := os.WriteFile(resolvedPath(renameRoomFile), []byte(payload), 0600); err != nil {
		log.Fatalf("failed to write rename file: %v", err)
	}
	fmt.Printf("Rename queued: #%s → #%s (will apply when server picks it up)\n", oldName, newName)
}

func runRemoveRoom(name string) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		fmt.Println("Room name cannot be empty.")
		os.Exit(1)
	}
	// Protect the landing room
	firstRoom := "lounge" // fallback
	if cfg, err := config.Load(resolvedPath("tavern.yaml")); err == nil {
		firstRoom = cfg.FirstRoom()
	}
	if name == firstRoom {
		fmt.Printf("Cannot remove the landing room #%s\n", name)
		os.Exit(1)
	}
	if err := os.WriteFile(resolvedPath(removeRoomFile), []byte(name), 0600); err != nil {
		log.Fatalf("failed to write remove file: %v", err)
	}
	fmt.Printf("Remove queued: #%s (users will be moved to #%s)\n", name, firstRoom)
}

func runBan(nickname string) {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" {
		fmt.Println("Nickname cannot be empty.")
		os.Exit(1)
	}
	st, err := store.New(resolvedDBPath())
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	fp, err := st.FingerprintByNickname(nickname)
	if err != nil {
		fmt.Printf("User %q not found.\n", nickname)
		os.Exit(1)
	}

	if err := st.Ban(fp, "banned by admin", nil); err != nil {
		log.Fatalf("ban failed: %v", err)
	}

	// Signal the server to kick them
	if err := os.WriteFile(resolvedPath(banFile), []byte(fp), 0600); err != nil {
		log.Fatalf("failed to write ban file: %v", err)
	}

	fmt.Printf("Banned %s (fingerprint: %s). They will be kicked if online.\n", nickname, fp[:16]+"...")
}

func runUnban(nickname string) {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" {
		fmt.Println("Nickname cannot be empty.")
		os.Exit(1)
	}
	st, err := store.New(resolvedDBPath())
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	fp, err := st.FingerprintByNickname(nickname)
	if err != nil {
		fmt.Printf("User %q not found.\n", nickname)
		os.Exit(1)
	}

	if err := st.Unban(fp); err != nil {
		log.Fatalf("unban failed: %v", err)
	}
	fmt.Printf("Unbanned %s.\n", nickname)
}

func runBanList() {
	st, err := store.New(resolvedDBPath())
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	bans, err := st.BanList()
	if err != nil {
		log.Fatalf("failed to list bans: %v", err)
	}
	if len(bans) == 0 {
		fmt.Println("No active bans.")
		return
	}

	fmt.Printf("%-20s %-20s %s\n", "NICKNAME", "BANNED AT", "FINGERPRINT")
	fmt.Println(strings.Repeat("─", 64))
	for _, b := range bans {
		fp := b.Fingerprint
		if len(fp) > 16 {
			fp = fp[:16] + "..."
		}
		fmt.Printf("%-20s %-20s %s\n", b.Nickname, b.BannedAt, fp)
	}
	fmt.Printf("\n%d ban(s) total.\n", len(bans))
}

func runSetFlag(wargameName, levelStr, flag string) {
	level, err := strconv.Atoi(levelStr)
	if err != nil || level < 1 {
		fmt.Println("Level must be a positive number.")
		os.Exit(1)
	}
	st, err := store.New(resolvedDBPath())
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()
	ws := wargame.New(st.DB())
	if err := ws.SetFlag(wargameName, level, flag); err != nil {
		log.Fatalf("set flag: %v", err)
	}
	fmt.Printf("Flag set: %s level %d\n", wargameName, level)
}

func runListFlags(wargameName string) {
	st, err := store.New(resolvedDBPath())
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()
	ws := wargame.New(st.DB())
	levels := ws.ListFlags(wargameName)
	if len(levels) == 0 {
		fmt.Printf("No flags set for %s\n", wargameName)
		return
	}
	fmt.Printf("%s flags (%d levels):\n", wargameName, len(levels))
	for _, l := range levels {
		fmt.Printf("  level %d  ✓\n", l)
	}
}

func runPurge() {
	// Broadcast purge to connected clients before wiping
	os.WriteFile(resolvedPath(purgeFile), []byte("1"), 0600)
	fmt.Println("Signaled connected clients...")
	time.Sleep(2 * time.Second) // give server time to broadcast

	st, err := store.New(resolvedDBPath())
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	fmt.Println("Purging all data (users, chat, gallery, visitors)...")
	fmt.Println("Bans and owners preserved.")
	if err := st.PurgeAll(); err != nil {
		log.Fatalf("purge failed: %v", err)
	}
	st.Close()

	fmt.Println("Restarting server...")
	cmd := exec.Command("sudo", "/usr/bin/systemctl", "restart", "tavrn")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("restart failed: %v (restart manually with: sudo systemctl restart tavrn)", err)
	} else {
		fmt.Println("Done. Server restarted with clean state.")
	}
}

func runServer() {
	// Redirect logs to file so they don't corrupt connected TUI sessions
	logFile, err := os.OpenFile("tavrn.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	// Load tavern config — required for startup
	configPath := resolvedPath("tavern.yaml")
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		fmt.Fprintln(os.Stderr, "ERROR: tavern.yaml not found.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "This is the tavern engine — you need to configure your own tavern.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  cp tavern.yaml.example tavern.yaml")
		fmt.Fprintln(os.Stderr, "  # edit tavern.yaml with your tavern name, domain, and SSH fingerprint")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "To find your SSH key fingerprint:")
		fmt.Fprintln(os.Stderr, "  ssh-keygen -lf ~/.ssh/id_ed25519.pub")
		os.Exit(1)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	sanitize.SetOwnerNick(cfg.Owner.Name)

	st, err := store.New(resolvedDBPath())
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()
	st.SeedRooms(cfg.RoomNames())

	h := hub.New()
	go h.Run()

	if _, err := os.Stat(".ssh"); os.IsNotExist(err) {
		os.MkdirAll(".ssh", 0700)
	}

	catalog := jukebox.NewCatalog()
	log.Printf("Tavern Radio: %d tracks loaded", catalog.TrackCount())
	jukeboxEngine := jukebox.NewEngineWithCatalog(catalog)
	jukeboxEngine.SetOnlineCount(h.OnlineCount)
	streamer := jukebox.NewStreamer()
	streamer.SetOnDurationKnown(func(seconds int) {
		jukeboxEngine.UpdateDuration(seconds)
	})
	streamer.SetOnError(func() {
		// Download failed — immediately retry with a new track
		jukeboxEngine.RetryTrack()
	})
	jukeboxEngine.SetOnTrackChange(func(track jukebox.Track) {
		streamer.StreamTrack(track)
	})

	sudokuGame := sudoku.NewGame("evil")
	log.Printf("Sudoku: evil puzzle ready (%d clues)", sudokuGame.Filled())

	// Web audio streaming
	if hasFlag("--web-audio") {
		ws := webstream.New(streamer, jukeboxEngine)
		go ws.ListenAndServe(":8090")
	}

	pollStore := poll.NewStore()

	// Bartender AI — disabled if OPENAI_API_KEY is not set
	var bt *bartender.Bartender
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey != "" {
		soulRaw, err := os.ReadFile(resolvedPath("bartender/soul.md"))
		if err != nil {
			log.Printf("bartender: soul.md not found, trying soul.md.example")
			soulRaw, err = os.ReadFile(resolvedPath("bartender/soul.md.example"))
			if err != nil {
				log.Printf("bartender: no soul file found, using default")
				soulRaw = []byte("You are a gruff bartender in a terminal tavern. Keep replies to 1-2 sentences.")
			}
		}

		soul := string(soulRaw)
		// Render template variables if present
		if tmpl, tmplErr := template.New("soul").Parse(soul); tmplErr == nil {
			var buf bytes.Buffer
			data := map[string]string{
				"TavernName": cfg.Tavern.Name,
				"OwnerName":  cfg.Owner.Name,
				"Domain":     cfg.Tavern.Domain,
			}
			if err := tmpl.Execute(&buf, data); err == nil {
				soul = buf.String()
			}
		}

		bt = bartender.New(apiKey, soul, st)
		log.Println("bartender: enabled")
	} else {
		log.Println("bartender: disabled (no OPENAI_API_KEY)")
	}

	port := getPort()
	// GIF search — disabled if KLIPY_API_KEY is not set
	var gifClient *gif.KlipyClient
	if klipyKey := os.Getenv("KLIPY_API_KEY"); klipyKey != "" {
		gifClient = gif.NewKlipyClient(klipyKey)
		log.Println("gif search: enabled")
	} else {
		log.Println("gif search: disabled (no KLIPY_API_KEY)")
	}

	// Web search for bartender — Exa with DuckDuckGo fallback
	var searcher *search.Searcher
	exaKey := os.Getenv("EXA_API_KEY")
	searcher = search.New(exaKey)
	if exaKey != "" {
		log.Println("web search: enabled (Exa + DuckDuckGo fallback)")
	} else {
		log.Println("web search: DuckDuckGo only (no EXA_API_KEY)")
	}

	srv, err := server.New(server.Config{
		Host:             "0.0.0.0",
		Port:             port,
		HostKeyPath:      ".ssh/id_ed25519",
		Store:            st,
		Hub:              h,
		JukeboxEngine:    jukeboxEngine,
		SudokuGame:       sudokuGame,
		PollStore:        pollStore,
		Bartender:        bt,
		TavernName:       cfg.Tavern.Name,
		TavernDomain:     cfg.Tavern.Domain,
		Tagline:          cfg.Tavern.Tagline,
		OwnerName:        cfg.Owner.Name,
		OwnerFingerprint: cfg.Owner.Fingerprint,
		FirstRoom:        cfg.FirstRoom(),
		RoomTypes:        cfg.RoomTypeMap(),
		GifClient:        gifClient,
		WargameStore:     wargame.New(st.DB()),
		Searcher:         searcher,
	})
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startPurgeScheduler(st, h, pollStore)
	if bt != nil {
		go func() {
			for {
				time.Sleep(5 * time.Minute)
				bt.DecayMood()
			}
		}()
	}
	go watchBannerFile(st, h)
	go watchAddRoomFile(st, h)
	go watchRenameRoomFile(st, h)
	go watchRemoveRoomFile(st, h)
	go watchBanFile(h)
	go watchPurgeFile(h)
	go watchBartenderToggle(bt)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.Start(ctx); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	log.Printf("%s is open. ssh localhost -p %d", cfg.Tavern.Domain, port)

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
	if err := runCommand(repoDir, env, "go", "build", "-o", "tavrn", "./cmd/tavrn-admin"); err != nil {
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

// resolvedPath returns the path to a file, checking the working directory
// first, then next to the executable. For new files that don't exist yet,
// it prefers cwd if tavern.yaml is there (indicates correct working dir).
func resolvedPath(name string) string {
	// If file exists in cwd, use that
	if _, err := os.Stat(name); err == nil {
		return name
	}
	// If cwd looks like the repo dir (has tavern.yaml), use cwd for new files too
	if _, err := os.Stat("tavern.yaml"); err == nil {
		return name
	}
	// Fall back to next to the executable
	repoDir, err := executableRepoDir()
	if err != nil {
		return name
	}
	return filepath.Join(repoDir, name)
}

func resolvedDBPath() string {
	return resolvedPath("tavrn.db")
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
	mergedPath := "PATH=" + strings.Join(pathParts, ":")
	base := os.Environ()
	env := make([]string, 0, len(base)+1)
	replaced := false
	for _, kv := range base {
		if strings.HasPrefix(kv, "PATH=") {
			if !replaced {
				env = append(env, mergedPath)
				replaced = true
			}
			continue
		}
		env = append(env, kv)
	}
	if !replaced {
		env = append(env, mergedPath)
	}
	return env
}

func runCommand(dir string, env []string, name string, args ...string) error {
	resolved, err := resolveCommand(name, env)
	if err != nil {
		return err
	}
	cmd := exec.Command(resolved, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func commandOutput(dir string, env []string, name string, args ...string) (string, error) {
	resolved, err := resolveCommand(name, env)
	if err != nil {
		return "", err
	}
	cmd := exec.Command(resolved, args...)
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

func resolveCommand(name string, env []string) (string, error) {
	if strings.Contains(name, "/") {
		return name, nil
	}

	pathEnv := os.Getenv("PATH")
	for i := len(env) - 1; i >= 0; i-- {
		kv := env[i]
		if strings.HasPrefix(kv, "PATH=") {
			pathEnv = strings.TrimPrefix(kv, "PATH=")
			break
		}
	}

	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			dir = "."
		}
		candidate := filepath.Join(dir, name)
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		if info.Mode()&0111 != 0 {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("exec: %q not found in PATH", name)
}

func startPurgeScheduler(st *store.Store, h *hub.Hub, ps *poll.Store) {
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
		ps.Clear()
		log.Println("Weekly purge complete")
	}
}

func watchBannerFile(st *store.Store, h *hub.Hub) {
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
		st.SetBanner(text)
		h.BroadcastAll(session.Msg{
			Type: session.MsgBanner,
			Text: text,
		})
	}
}

func watchAddRoomFile(st *store.Store, h *hub.Hub) {
	for {
		time.Sleep(1 * time.Second)

		data, err := os.ReadFile(addRoomFile)
		if err != nil {
			continue
		}

		name := strings.ToLower(strings.TrimSpace(string(data)))
		if name == "" {
			continue
		}

		os.Remove(addRoomFile)

		if st.IsRoom(name) {
			log.Printf("Room #%s already exists", name)
			continue
		}

		if err := st.AddRoom(name); err != nil {
			log.Printf("Failed to add room: %v", err)
			continue
		}

		log.Printf("Room added: #%s", name)
		h.BroadcastAll(session.Msg{
			Type: session.MsgRoomAdded,
			Text: name,
		})
	}
}

func watchRenameRoomFile(st *store.Store, h *hub.Hub) {
	for {
		time.Sleep(1 * time.Second)

		data, err := os.ReadFile(renameRoomFile)
		if err != nil {
			continue
		}

		payload := strings.TrimSpace(string(data))
		os.Remove(renameRoomFile)

		parts := strings.SplitN(payload, ":", 2)
		if len(parts) != 2 {
			log.Printf("Invalid rename payload: %q", payload)
			continue
		}
		oldName := strings.ToLower(parts[0])
		newName := strings.ToLower(parts[1])

		if !st.IsRoom(oldName) {
			log.Printf("Room #%s does not exist", oldName)
			continue
		}
		if st.IsRoom(newName) {
			log.Printf("Room #%s already exists", newName)
			continue
		}

		if err := st.RenameRoom(oldName, newName); err != nil {
			log.Printf("Failed to rename room: %v", err)
			continue
		}

		log.Printf("Room renamed: #%s → #%s", oldName, newName)
		h.BroadcastAll(session.Msg{
			Type: session.MsgRoomRenamed,
			Text: oldName,
			Room: newName,
		})
	}
}

func watchRemoveRoomFile(st *store.Store, h *hub.Hub) {
	for {
		time.Sleep(1 * time.Second)

		data, err := os.ReadFile(removeRoomFile)
		if err != nil {
			continue
		}

		name := strings.ToLower(strings.TrimSpace(string(data)))
		if name == "" {
			continue
		}
		os.Remove(removeRoomFile)

		if !st.IsRoom(name) {
			log.Printf("Room #%s does not exist", name)
			continue
		}

		if err := st.DeleteRoom(name); err != nil {
			log.Printf("Failed to remove room: %v", err)
			continue
		}

		log.Printf("Room removed: #%s", name)
		h.BroadcastAll(session.Msg{
			Type: session.MsgRoomRemoved,
			Text: name,
		})
	}
}

func watchBanFile(h *hub.Hub) {
	for {
		time.Sleep(1 * time.Second)

		data, err := os.ReadFile(banFile)
		if err != nil {
			continue
		}

		fp := strings.TrimSpace(string(data))
		if fp == "" {
			continue
		}
		os.Remove(banFile)

		if h.Kick(fp) {
			log.Printf("Kicked banned user: %s", fp[:16]+"...")
		}
	}
}

func watchPurgeFile(h *hub.Hub) {
	for {
		time.Sleep(1 * time.Second)

		if _, err := os.ReadFile(purgeFile); err != nil {
			continue
		}

		os.Remove(purgeFile)

		log.Println("Manual purge signal received, broadcasting to clients")
		h.BroadcastAll(session.Msg{
			Type: session.MsgPurge,
		})
	}
}

func watchBartenderToggle(bt *bartender.Bartender) {
	for {
		time.Sleep(1 * time.Second)

		data, err := os.ReadFile(bartenderToggleFile)
		if err != nil {
			continue
		}

		os.Remove(bartenderToggleFile)

		if bt == nil {
			log.Println("bartender: toggle ignored (not initialized)")
			continue
		}

		action := strings.TrimSpace(string(data))
		switch action {
		case "off":
			bt.Disable()
			log.Println("bartender: disabled by admin")
		case "on":
			bt.Enable()
			log.Println("bartender: enabled by admin")
		}
	}
}
