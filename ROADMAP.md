# Roadmap

## Tavern AI — The Bartender

A local AI that lives in the tavern and learns from the regulars.

- Runs a local model on the server — no cloud, no filters, no censorship
- Users tag it in chat with `@barkeep` to ask questions or just talk
- Learns tone, slang, and humor from daily conversations over time
- Personality evolves based on who hangs out — the tavern shapes the AI
- Fine-tunes on chat history: starts generic, becomes a local character
- Inspired by Grok's unfiltered approach but trained by the tavern's own crowd
- Could serve as trivia host, storyteller, argument settler, or just another regular

---

## Theater Room — Watch YouTube Together

A shared room where users paste YouTube links, vote on what plays next, and watch together.

- Audio streams via the website player
- Server uses yt-dlp for metadata and audio
- Voting system: paste link, upvote, most votes plays next
- TUI shows now-playing info, queue, and vote counts while users chat

## Tavern Games

Multiplayer terminal games that fit the tavern theme.

- `/roll 2d6` — dice rolling for tabletop RPG sessions
- Trivia — timed questions, scoreboard, themed rounds
- Word games — hangman, word chains
- Tic-tac-toe, connect four — challenge another user
- Text adventure — room votes on choices, story unfolds together

## Hacker News / Reddit Reader

A room where users browse and discuss threads together.

- HN: public JSON API, no auth needed
- Reddit: scrape old.reddit.com/.json, no API key
- Scrollable thread view in the TUI
- Everyone reads the same thread and discusses in chat
- `/hn top` `/hn new` `/reddit r/golang` to navigate

## Mastodon Feed

Public Mastodon timeline in a dedicated room.

- Public API, no auth required for public posts
- Render toots with author, content, boosts
- Users discuss posts in real-time chat
- `/fedi trending` `/fedi local instance.social`

## Radio Requests + Voting

Let users browse the catalog and queue tracks on the website.

- Browse tracks on the website
- Request a track — goes into the queue
- Other users upvote requests
- Most-voted track plays next
- Falls back to random if queue is empty

## DMs

Private messages between users.

- `/dm @nickname message` to whisper
- Conversation appears in a side panel or modal
- Routed by SSH fingerprint, no accounts needed

## Collaborative ASCII Canvas

A shared drawing room — r/place in the terminal.

- Grid canvas, users move cursor and place characters
- Color support via ANSI
- Canvas persists until weekly reset
- Watch others draw in real-time

## Federation — Self-Hosted Taverns

Run your own tavern. Join the network.

- One binary, one config — spin up your own tavern on your own server
- Standalone mode: fully isolated, your rules, your rooms, your music
- Each tavern keeps its own identity — custom name, rooms, catalogs, AI personality
- MIT licensed: fork it, run it, make it yours

### The Network — tavrn.sh as the Registry

tavrn.sh is the directory, not the host. Tavern owners run their own servers.

- Register your tavern by submitting a PR to the main repo with your tavern's info
- A `taverns.json` catalog lists all known taverns (name, address, description)
- Get a vanity subdomain: `myown.tavrn.sh` DNS points to your server's IP
- `ssh myown.tavrn.sh` connects directly to the owner's box
- From the main tavern, `/visit myown` hops you there
- tavrn.sh displays a bulletin board of all registered taverns with live status
- SSH key is your passport — same identity across the entire network

### How to Join the Network

1. Fork the repo, deploy your tavern (MIT license)
2. Submit a PR adding your tavern to `taverns.json` (name, host, description)
3. PR gets reviewed — tavern must be running and reachable
4. Once merged, your tavern appears on the bulletin board and gets a `*.tavrn.sh` subdomain
