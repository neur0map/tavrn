package room

import "testing"

func TestValidTypes(t *testing.T) {
	expected := []string{"chat", "gallery", "games"}
	for _, rt := range expected {
		if !ValidTypes[rt] {
			t.Errorf("expected %q to be a valid type", rt)
		}
	}
	if ValidTypes["invalid"] {
		t.Error("invalid should not be a valid type")
	}
}
