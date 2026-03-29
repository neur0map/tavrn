package webstream

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"tavrn.sh/internal/jukebox"
)

type Handler struct {
	streamer *jukebox.Streamer
	engine   *jukebox.Engine
}

func New(streamer *jukebox.Streamer, engine *jukebox.Engine) *Handler {
	return &Handler{streamer: streamer, engine: engine}
}

// NowPlaying returns the current track info as JSON.
func (h *Handler) NowPlaying(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	state := h.engine.State()
	resp := map[string]any{
		"playing": false,
	}
	if state.Current != nil {
		resp["playing"] = true
		resp["title"] = state.Current.Title
		resp["artist"] = state.Current.Artist
		resp["duration"] = state.Current.Duration
		resp["position"] = int(state.Position.Seconds())
	}
	json.NewEncoder(w).Encode(resp)
}

// Stream serves continuous MP3 audio to the browser.
func (h *Handler) Stream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Subscribe to track changes
	sub := h.streamer.SubscribeTrackChange()
	defer h.streamer.UnsubscribeTrackChange(sub)

	// bufferEnd estimates when the client will exhaust its buffered audio.
	var bufferEnd time.Time

	// Send current track from current position
	track, audio, playStart := h.streamer.CurrentAudio()
	if track != nil && len(audio) > 0 {
		// Calculate byte offset based on elapsed time
		elapsed := time.Since(playStart).Seconds()
		duration := float64(track.Duration)
		if duration > 0 {
			progress := elapsed / duration
			if progress > 1.0 {
				progress = 1.0
			}
			skip := int(progress * float64(len(audio)))
			audio = audio[skip:]
		}
		if _, err := w.Write(audio); err != nil {
			return
		}
		flusher.Flush()
		remaining := duration - elapsed
		if remaining < 0 {
			remaining = 0
		}
		bufferEnd = time.Now().Add(time.Duration(remaining * float64(time.Second)))
	}

	// Keepalive: feed silent MP3 frames when the client buffer is nearly
	// empty so the browser never thinks the stream has ended.
	keepalive := time.NewTicker(500 * time.Millisecond)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case _, ok := <-sub:
			if !ok {
				return
			}
			track, audio, _ := h.streamer.CurrentAudio()
			if track == nil || len(audio) == 0 {
				continue
			}
			if _, err := w.Write(audio); err != nil {
				return
			}
			flusher.Flush()
			bufferEnd = time.Now().Add(time.Duration(track.Duration) * time.Second)
		case <-keepalive.C:
			// Send silence when the client is within 5s of running dry,
			// or if no track has been sent yet.
			if bufferEnd.IsZero() || time.Now().After(bufferEnd.Add(-5*time.Second)) {
				if _, err := w.Write(SilentFrame); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}

// ServeMux returns a configured HTTP mux.
func (h *Handler) ServeMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/stream", h.Stream)
	mux.HandleFunc("/now-playing", h.NowPlaying)
	return mux
}

// ListenAndServe starts the web audio HTTP server.
func (h *Handler) ListenAndServe(addr string) error {
	log.Printf("Web audio streaming on %s", addr)
	return http.ListenAndServe(addr, h.ServeMux())
}
