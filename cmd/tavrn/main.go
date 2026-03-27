package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tavrn/internal/hub"
	"tavrn/internal/jukebox"
	"tavrn/internal/server"
	"tavrn/internal/session"
	"tavrn/internal/store"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "purge":
			runPurge()
			return
		case "help", "--help", "-h":
			fmt.Println("Usage:")
			fmt.Println("  tavrn          Start the SSH server")
			fmt.Println("  tavrn purge    Purge all data (run from VPS terminal)")
			return
		}
	}

	runServer()
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

	srv, err := server.New(server.Config{
		Host:          "0.0.0.0",
		Port:          2222,
		HostKeyPath:   ".ssh/id_ed25519",
		Store:         st,
		Hub:           h,
		JukeboxEngine: jukeboxEngine,
	})
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Schedulers
	go startPurgeScheduler(st, h)
	go startGalleryCleanup(st, h)

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
