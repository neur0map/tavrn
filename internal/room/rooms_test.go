package room

import "testing"

func TestDefaults(t *testing.T) {
	if len(Defaults) == 0 {
		t.Fatal("expected at least one default room")
	}
	expected := map[string]bool{"lounge": true, "gallery": true, "games": true, "suggestions": true}
	for _, r := range Defaults {
		if !expected[r] {
			t.Errorf("unexpected default room: %q", r)
		}
	}
	for name := range expected {
		found := false
		for _, r := range Defaults {
			if r == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing default room: %q", name)
		}
	}
}
