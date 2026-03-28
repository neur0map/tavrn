# Lofi Jukebox Rework

## Summary

Strip the jukebox down to lofi-only. Remove Jamendo, YouTube, search, multi-backend machinery. Replace with a single-screen modal showing now playing + vote-to-skip.

## Layout

```
╱╱╱╱╱╱╱╱╱╱╱╱╱ ♪ Lofi Radio ╱╱╱╱╱╱╱╱╱╱╱╱╱

  Such Great Heights
  Chillhop

  ▓▓▓▓▓▓▓▓▓▓▓░░░░░░░░░░░░░░░░░░  1:17/3:27

  ● playing · 4 listening

╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱╱

  S skip (2 more votes needed)   ESC close
```

After voting: `S skip ✓ (1 more vote needed)`
Skip voters reset on every new track.

## Skip threshold

```
if online <= 5 → 1 (instant)
if online >= 6 → 3 + (online - 6) / 10
```

Edge case: if people disconnect and threshold drops below existing votes, skip triggers on next tick.

## Remove

- `jamendo.go`, `jamendo_test.go`
- `youtube.go`, `youtube_test.go`
- `MusicBackend` interface
- `Engine.backends`, `tryAutoPlay()`, `Backends()`
- Search tab, Vote tab, all search/vote messages
- `jukeboxSearch()` in app.go
- CTRL+L shortcut
- `JAMENDO_CLIENT_ID` env var handling

## Keep

- `Engine` — simplified: play, duration, skip
- `Streamer` — unchanged
- `Lofi` struct with both catalogs
- `ffprobe` duration detection
- `lofi_catalog.go` + both `.txt` catalogs

## New

- `Engine.VoteSkip(fingerprint string) bool` — returns true if skip triggered
- `Engine.SkipState() (votes, threshold int)`
- `Engine.SetOnlineCount(func() int)` — live count for threshold
- `session.MsgSkipVote` — broadcast skip state
- Simplified `JukeboxModal` — single screen, S to skip

## Engine flow

1. Startup → random track → play
2. Streamer downloads, ffprobe gets duration, broadcasts
3. Duration expires → next random track
4. Download fails → try another random track immediately
5. No idle phase — always playing

## Duration

- Tracks start with `Duration: 0`
- Engine waits for `UpdateDuration()` from streamer ffprobe callback
- Progress bar shows once duration is known
