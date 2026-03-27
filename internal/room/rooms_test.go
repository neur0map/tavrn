package room

import "testing"

func TestAllRooms(t *testing.T) {
	if len(All) == 0 {
		t.Fatal("expected at least one room")
	}
	expected := map[string]bool{"lounge": true, "gallery": true, "suggestions": true}
	for _, r := range All {
		if !expected[r] {
			t.Errorf("unexpected room: %q", r)
		}
	}
	for name := range expected {
		found := false
		for _, r := range All {
			if r == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing room: %q", name)
		}
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"lounge", true},
		{"gallery", true},
		{"suggestions", true},
		{"nonexistent", false},
		{"", false},
		{"LOUNGE", false},
	}
	for _, tt := range tests {
		if got := IsValid(tt.name); got != tt.valid {
			t.Errorf("IsValid(%q) = %v, want %v", tt.name, got, tt.valid)
		}
	}
}
