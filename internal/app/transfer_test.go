package app

import (
	"strings"
	"testing"
)

func TestNextMatch(t *testing.T) {
	text := "La maison est grande, la maison est belle"

	start, end, found := nextMatch(text, "maison", 0)
	if !found || start != 3 || end != 9 {
		t.Errorf("nextMatch from 0: expected (3, 9, true), got (%d, %d, %v)", start, end, found)
	}

	// Recherche à partir de l'occurrence trouvée (position juste après)
	// "La maison est grande, la maison est belle" -> 2e "maison" à 25
	start, end, found = nextMatch(text, "maison", 9)
	if !found || start != 25 || end != 31 {
		t.Errorf("nextMatch from 9: expected (25, 31, true), got (%d, %d, %v)", start, end, found)
	}

	// Insensible à la casse
	start, end, found = nextMatch(text, "MAISON", 0)
	if !found || start != 3 || end != 9 {
		t.Errorf("case-insensitive: expected (3, 9, true), got (%d, %d, %v)", start, end, found)
	}

	// Aucune occurrence
	_, _, found = nextMatch(text, "chateau", 0)
	if found {
		t.Error("expected no match for 'chateau'")
	}

	// Position au-delà de la fin du texte
	_, _, found = nextMatch(text, "maison", len(text)+10)
	if found {
		t.Error("expected no match when from >= len(text)")
	}

	// Terme vide
	_, _, found = nextMatch(text, "", 0)
	if found {
		t.Error("expected no match for empty term")
	}

	// Position négative est ramenée à 0
	start, _, found = nextMatch(text, "maison", -5)
	if !found || start != 3 {
		t.Errorf("negative from: expected start 3, got %d (found=%v)", start, found)
	}
}

func TestProgressState(t *testing.T) {
	state := &progressState{}
	state.addFile()
	state.addFile()
	state.fileDone()
	state.addBytes(1024)
	state.addBytes(512)

	if state.filesTotal != 2 || state.filesCopied != 1 {
		t.Errorf("expected filesTotal=2 filesCopied=1, got %d/%d", state.filesTotal, state.filesCopied)
	}
	if state.bytesCopied != 1536 {
		t.Errorf("expected bytesCopied=1536, got %d", state.bytesCopied)
	}
	if got := state.summary(); !strings.Contains(got, "1/2") || !strings.Contains(got, "1.5 KB") {
		t.Errorf("unexpected summary: %q", got)
	}
}
