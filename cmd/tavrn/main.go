package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"tavrn/internal/hub"
	"tavrn/internal/jukebox"
	"tavrn/internal/server"
	"tavrn/internal/session"
	"tavrn/internal/store"
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
		case "help", "--help", "-h":
			fmt.Println("Usage:")
			fmt.Println("  tavrn                       Start the SSH server")
			fmt.Println("  tavrn purge                 Purge all data")
			fmt.Println("  tavrn --message \"text\"       Send banner to all connected users")
			return
		}
	}

	runServer()
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

	srv, err := server.New(server.Config{
		Host:          "0.0.0.0",
		Port:          2222,
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

	// Schedulers
	go startPurgeScheduler(st, h)
	go startGalleryCleanup(st, h)
	go watchBannerFile(h)

	// Start SSH server
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.Start(ctx); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	log.Println("tavrn.sh is open. ssh localhost -p 2222")

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

		// Remove file immediately so it doesn't re-broadcast
		os.Remove(bannerFile)

		log.Printf("Broadcasting banner: %s", text)
		h.BroadcastAll(session.Msg{
			Type: session.MsgBanner,
			Text: text,
		})
	}
}
