# Multiplayer Sudoku — Design

## Overview

Collaborative Sudoku in the `#games` room. All players see and interact with the same board. Standard 9x9 with hard/evil difficulty. Split layout: board left, mini chat right.

## Scoring

- Place a correct number: **+1**
- Place a wrong number: **-1** (player doesn't know which until check)
- Check board (`C`): costs **3 points**, must have 3+ to use
- Check reveals all wrong cells (flash red for 5 seconds)
- Each wrong cell's placer loses **1 point** per wrong cell
- Scores can go negative
- Completion banner shows final scoreboard ranked by points

## Game State

```
SudokuGame
├── puzzle    [9][9]int       // starting clues (0 = empty)
├── solution  [9][9]int       // the answer key
├── board     [9][9]Cell      // current state
├── scores    map[fp]int      // player scores
├── cursors   map[fp]Position // player cursor positions
├── started   time.Time
├── difficulty string          // easy/medium/hard/evil
```

Each `Cell`: `value int`, `placedBy string`, `isClue bool`.

## Puzzle Generation

Pure Go, no dependencies. Backtracking algorithm:

1. Fill a valid 9x9 grid using recursive backtracking with random candidate order
2. Copy as solution
3. Remove cells based on difficulty (easy: 45 removed, medium: 50, hard: 55, evil: 60+)
4. After each removal, verify puzzle still has a unique solution
5. If not unique, put the cell back and try another

Difficulty is evil by default (best for multiple players).

## Message Types

- `MsgSudokuPlace` — player places a number: {Row, Col, Value, Fingerprint}
- `MsgSudokuClear` — player clears a cell: {Row, Col, Fingerprint}
- `MsgSudokuCheck` — check request + response with wrong cell positions
- `MsgSudokuCursor` — cursor position broadcast for multiplayer cursors
- `MsgSudokuNew` — vote for new puzzle / new puzzle generated
- `MsgSudokuState` — full board sync for late joiners

## Placement Flow

1. Player presses `5` on cell R3C7
2. Client sends `MsgSudokuPlace{Row:3, Col:7, Value:5}`
3. Server validates: not a clue cell, value 1-9
4. Compares against solution — correct +1, wrong -1
5. Number placed on board with `placedBy` set (either way)
6. Broadcasts updated cell + score to all in `#games`

## Check Flow

1. Player presses `C`, must have 3+ points
2. Server deducts 3 from that player
3. Scans board for `value != solution` on non-clue cells
4. Broadcasts wrong cell positions to all clients
5. Clients flash wrong cells red for 5 seconds
6. Each wrong cell's placer loses 1 point
7. Wrong cells stay — anyone can overwrite them

## Layout

```
╭─ #games ──────────────────────────────────────────────────────╮
│                                                               │
│   ┌───────┬───────┬───────┐                                   │
│   │ 5 · · │ · 8 · │ · · 9 │    ── Game Chat ──────────────   │
│   │ · · 3 │ · · · │ 1 · · │    dusty#2847  2s ago            │
│   │ · · · │ 7 · · │ · · 4 │      try row 3?                  │
│   ├───────┼───────┼───────┤                                   │
│   │ · · 5 │ · · · │ · 7 · │    lone#0931  5s ago             │
│   │ · 8 · │ · 1 · │ · 3 · │      its definitely a 6         │
│   │ · 2 · │ · · · │ 9 · · │                                   │
│   ├───────┼───────┼───────┤    ────────────────────────────   │
│   │ 3 · · │ · · 8 │ · · · │    → > _                         │
│   │ · · 7 │ · · · │ 2 · · │                                   │
│   │ 8 · · │ · 3 · │ · · 1 │                                   │
│   └───────┴───────┴───────┘                                   │
│                                                               │
│   dusty#2847: 12  ·  lone#0931: 5  ·  Evil · 31/81           │
│   ←→↑↓ move  1-9 place  x clear  C check(3)  Tab chat  ESC  │
╰───────────────────────────────────────────────────────────────╯
```

## Controls

- `←→↑↓` / `hjkl` — move cursor
- `1-9` — place number at cursor
- `x` / `backspace` — clear own placement
- `C` — check board (3 points)
- `N` — vote for new puzzle (50%+ required)
- `Tab` — toggle focus between board and chat
- `ESC` — leave to lounge

## Visual Style

- Clue numbers: bold white (immutable)
- Player placements: rendered in player's nick color
- Wrong cells on check: red flash for 5 seconds
- Empty cells: `·`
- Own cursor: highlighted background
- Other cursors: dim colored underline

## Files

```
internal/sudoku/
  puzzle.go        — generation, solver, validation
  puzzle_test.go   — generation and solver tests
  game.go          — shared game state, scoring, placement
  game_test.go     — scoring and placement tests
ui/
  sudoku_view.go   — board rendering, mini chat, controls
```

## Room Integration

- `#games` added to seed rooms
- When room == "games", app renders `SudokuView` instead of `ChatView`
- Game state stored in memory (same as jukebox engine)
- New puzzle auto-generates on completion or majority vote
- Late joiners receive full board state via `MsgSudokuState`

## New Puzzle

- Auto-generates when puzzle is solved (scores persist across puzzles)
- `N` key votes for new puzzle — needs 50%+ of players in room
- Scores reset on new puzzle
- Completion banner shows final rankings before reset
