package mystery

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testCaseDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "mysteries", "case01")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("test case data not found at %s", dir)
	}
	return dir
}

func TestNew(t *testing.T) {
	e, err := New(testCaseDir(t))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if len(e.triggers) == 0 {
		t.Fatal("expected triggers to be loaded")
	}
	if e.solution == "" {
		t.Fatal("expected solution to be set")
	}
	if len(e.fakeNames) != fakeNameCount {
		t.Fatalf("expected %d fake names, got %d", fakeNameCount, len(e.fakeNames))
	}
	if !e.IsActive() {
		t.Fatal("expected IsActive() to be true")
	}
}

func TestCheckTriggersClue(t *testing.T) {
	e, err := New(testCaseDir(t))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	r := e.Check("anyone hear about the murder?")
	if r == nil {
		t.Fatal("expected result, got nil")
	}
	if len(r.Clues) == 0 {
		t.Fatal("expected at least one clue")
	}
	if r.Sender == "" {
		t.Fatal("expected non-empty sender")
	}
	if r.Solved {
		t.Fatal("should not be solved")
	}
}

func TestCheckNoRepeatClues(t *testing.T) {
	e, err := New(testCaseDir(t))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	r1 := e.Check("murder")
	if r1 == nil || len(r1.Clues) == 0 {
		t.Fatal("expected clue on first trigger")
	}

	// Reset cooldown so second check isn't blocked by timing
	e.mu.Lock()
	e.lastClueAt = time.Time{}
	e.mu.Unlock()

	r2 := e.Check("murder")
	if r2 != nil {
		t.Fatal("expected nil on repeat keyword, got result")
	}
}

func TestCheckEmptyInput(t *testing.T) {
	e, err := New(testCaseDir(t))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if r := e.Check(""); r != nil {
		t.Fatal("expected nil for empty input")
	}
	if r := e.Check("   "); r != nil {
		t.Fatal("expected nil for whitespace input")
	}
}

func TestCheckSolve(t *testing.T) {
	e, err := New(testCaseDir(t))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	r := e.Check("I think it was jeremy bowers")
	if r == nil {
		t.Fatal("expected result, got nil")
	}
	if !r.Solved {
		t.Fatal("expected Solved to be true")
	}
	if r.Killer == "" {
		t.Fatal("expected killer name")
	}
	if len(r.Confession) == 0 {
		t.Fatal("expected confession lines")
	}
}

func TestCheckSolveOnce(t *testing.T) {
	e, err := New(testCaseDir(t))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	r1 := e.Check("jeremy bowers")
	if r1 == nil || !r1.Solved {
		t.Fatal("expected solve on first attempt")
	}

	r2 := e.Check("jeremy bowers")
	if r2 != nil {
		t.Fatal("expected nil on second solve attempt")
	}
}

func TestCheckCooldown(t *testing.T) {
	e, err := New(testCaseDir(t))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	r1 := e.Check("murder")
	if r1 == nil {
		t.Fatal("expected first trigger to work")
	}

	// Immediately try another keyword — should be blocked by cooldown
	r2 := e.Check("body")
	if r2 != nil {
		t.Fatal("expected nil due to cooldown")
	}
}

func TestReset(t *testing.T) {
	e, err := New(testCaseDir(t))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	r1 := e.Check("murder")
	if r1 == nil || len(r1.Clues) == 0 {
		t.Fatal("expected clue on first trigger")
	}

	e.Reset()

	// After reset, same keyword should trigger again
	r2 := e.Check("murder")
	if r2 == nil || len(r2.Clues) == 0 {
		t.Fatal("expected clue after reset")
	}
}
