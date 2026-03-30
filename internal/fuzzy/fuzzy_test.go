package fuzzy

import "testing"

func TestScore_ExactMatch(t *testing.T) {
	s := Score("deadmau5", "deadmau5")
	if s < 0 {
		t.Error("exact match should score positive")
	}
}

func TestScore_Prefix(t *testing.T) {
	s := Score("dead", "deadmau5")
	if s < 0 {
		t.Error("prefix should match")
	}
}

func TestScore_FuzzyInOrder(t *testing.T) {
	s := Score("dmu", "deadmau5")
	if s < 0 {
		t.Error("fuzzy in-order should match")
	}
}

func TestScore_NoMatch(t *testing.T) {
	s := Score("xyz", "deadmau5")
	if s >= 0 {
		t.Error("no match should return negative")
	}
}

func TestScore_CaseInsensitive(t *testing.T) {
	s := Score("DEAD", "deadmau5")
	if s < 0 {
		t.Error("case insensitive should match")
	}
}

func TestScore_PrefixBetterThanFuzzy(t *testing.T) {
	prefix := Score("dead", "deadmau5")
	fuzzy := Score("dmu5", "deadmau5")
	if prefix <= fuzzy {
		t.Errorf("prefix (%d) should score higher than fuzzy (%d)", prefix, fuzzy)
	}
}

func TestScore_ExactBetterThanPrefix(t *testing.T) {
	exact := Score("deadmau5", "deadmau5")
	prefix := Score("dead", "deadmau5")
	if exact <= prefix {
		t.Errorf("exact (%d) should score higher than prefix (%d)", exact, prefix)
	}
}

func TestScore_ContiguousBetterThanFuzzy(t *testing.T) {
	contig := Score("mau", "deadmau5")
	fuzzy := Score("dmu", "deadmau5")
	if contig <= fuzzy {
		t.Errorf("contiguous (%d) should score higher than fuzzy (%d)", contig, fuzzy)
	}
}

func TestScore_EmptyQuery(t *testing.T) {
	s := Score("", "deadmau5")
	if s < 0 {
		t.Error("empty query should match everything")
	}
}
