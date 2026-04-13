# Contributing

## Local testing

```bash
# Terminal 1 — server
go run ./cmd/tavrn-admin

# Terminal 2 — connect via SSH
ssh localhost -p 2222
```

## Branch workflow

```
feature/* ──PR──> dev ──merge──> main (deploy)
```

| Branch | Purpose |
|--------|---------|
| `main` | Production. Runs on the VPS. |
| `dev` | All development. PRs target here. |
| `feature/*` | Short-lived feature branches created from dev. |

1. Create a feature branch from `dev`
2. Open a PR targeting `dev`
3. Test locally
4. When dev is stable, merge to `main` during planned downtime

## Project structure

```
cmd/
  tavrn-admin/     Server binary (SSH server, jukebox engine)
internal/
  chat/            Message parsing and storage types
  hub/             Connection management, broadcasting
  identity/        Nickname generation, flair, color assignment
  jukebox/         Track catalog, engine, streamer (web audio)
  ratelimit/       Chat rate limiting
  room/            Room definitions
  sanitize/        Input sanitization
  server/          Wish SSH server setup
  session/         Session state, message types
  store/           SQLite persistence
  sudoku/          Multiplayer sudoku game logic
  webstream/       Web audio streaming handler
ui/
  app.go           Main Bubble Tea model
  modal.go         Modal system (help, nick, rooms)
  topbar.go        Top bar with room and stats
  sidebar.go       Rooms panel, online users
  chatview.go      Chat message rendering
  gallery.go       Sticky note board
  sudoku_view.go   Multiplayer sudoku view
  overlay.go       Modal overlay compositor
  styles.go        Cantina color palette
  splash.go        Welcome screen
```

## Architecture

**Server** — Wish-based SSH server. Each connection gets a Bubble Tea TUI. A shared hub broadcasts messages between sessions. The jukebox engine manages track playback state for web streaming.

**Web audio** — When started with `--web-audio`, the server runs an HTTP endpoint on `:8090` serving `/stream` (continuous MP3) and `/now-playing` (JSON metadata). Caddy reverse-proxies these to the public domain.

## Admin commands

All commands are run via the server binary (`tavrn-admin` or `go run ./cmd/tavrn-admin`):

```bash
# Server
tavrn-admin                              # Start the SSH server
tavrn-admin --web-audio                  # Start with web audio streaming on :8090
tavrn-admin --update                     # Pull main, rebuild, restart service

# Announcements
tavrn-admin --message "text"             # Send banner to all connected users
tavrn-admin --clear-banner               # Clear the active banner

# Rooms (live, no restart needed)
tavrn-admin --add-room "name"            # Add a new room
tavrn-admin --rename-room "old" "new"    # Rename a room
tavrn-admin --remove-room "name"         # Remove a room (moves users to landing room)

# Moderation
tavrn-admin --ban "nickname"             # Ban a user by nickname (kicks them)
tavrn-admin --unban "nickname"           # Unban a user
tavrn-admin --ban-list                   # Show all active bans

# Bartender
tavrn-admin --bartender-off              # Disable bartender (live)
tavrn-admin --bartender-on               # Enable bartender (live)

# Reddit feed
tavrn-admin --feed-add sub [sub...]      # Add subreddit(s) to feed
tavrn-admin --feed-remove sub            # Remove a subreddit from feed
tavrn-admin --feed-list                  # List configured subreddits

# Wargame CTF
tavrn-admin --set-flag bandit 1 "flag"   # Set a wargame flag
tavrn-admin --list-flags bandit          # List levels with flags

# Data
tavrn-admin purge                        # Purge all data (preserves bans and owners)
```

## Tests

```bash
go test ./...
```
