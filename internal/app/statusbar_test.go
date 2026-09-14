package app

import (
	"strings"
	"unicode/utf8"

	"testing"
)

// plainText retire les balises de couleur tview ([color]) du texte rendu
// pour obtenir le texte réellement visible.
func plainText(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '[' {
			// Balise de couleur tview : consomme jusqu'à la parenthèse fermante.
			if j := strings.IndexByte(s[i:], ']'); j >= 0 {
				i += j + 1
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		sb.WriteRune(r)
		i += size
	}
	return sb.String()
}

// visibleWidth renvoie la largeur écran du texte rendu (tous les libellés
// de la barre sont des caractères étroits : nombre de runes = largeur).
func visibleWidth(s string) int {
	return utf8.RuneCountInString(plainText(s))
}

func TestRenderBindingsBarWideTerminal(t *testing.T) {
	for _, mode := range []string{statusModeDefault, statusModeDual, statusModeArchive, statusModeEdit, statusModeView} {
		got := plainText(renderBindingsBar(mode, 500))
		if strings.ContainsRune(got, '…') {
			t.Errorf("%s: terminal large, troncature inattendue: %q", mode, got)
		}
		for _, item := range statusBarItems(mode) {
			if !strings.Contains(got, item.key+": "+item.label) {
				t.Errorf("%s: raccourci manquant %q dans %q", mode, item.key, got)
			}
		}
	}
}

func TestRenderBindingsBarNarrowTerminal(t *testing.T) {
	got := renderBindingsBar(statusModeDefault, 80)
	plain := plainText(got)

	// La largeur visible (hors balises) ne doit pas dépasser le terminal
	// (hors le « … » de troncature, 2 colonnes).
	if w := visibleWidth(got); w > 82 {
		t.Errorf("largeur visible %d > 82: %q", w, got)
	}

	// Les raccourcis prioritaires restent affichés, les secondaires sont coupés.
	for _, want := range []string{"F1: Aide", "F2: Tri", "F3: FTP"} {
		if !strings.Contains(plain, want) {
			t.Errorf("raccourci prioritaire manquant %q dans %q", want, plain)
		}
	}
	for _, absent := range []string{"F8: Double", "F9/Ctrl+H: Cachés", "Suppr: Effacer"} {
		if strings.Contains(plain, absent) {
			t.Errorf("raccourci secondaire présent alors qu'il devrait être coupé: %q dans %q", absent, plain)
		}
	}

	// Un « … » signale la troncature.
	if !strings.HasSuffix(plain, "…") {
		t.Errorf("troncature sans indicateur « … »: %q", plain)
	}
}

func TestRenderBindingsBarOrderPreserved(t *testing.T) {
	got := plainText(renderBindingsBar(statusModeDefault, 500))
	pos := -1
	for _, item := range statusBarItems(statusModeDefault) {
		idx := strings.Index(got, item.key+": "+item.label)
		if idx < 0 {
			t.Fatalf("raccourci manquant %q dans %q", item.key, got)
		}
		if idx < pos {
			t.Fatalf("ordre des raccourcis altéré à %q dans %q", item.key, got)
		}
		pos = idx
	}
}

func TestRenderBindingsBarUnknownWidth(t *testing.T) {
	// Avant le premier dessin, la largeur est inconnue (0) : pas de troncature.
	got := renderBindingsBar(statusModeEdit, 0)
	if strings.ContainsRune(got, '…') {
		t.Errorf("troncature inattendue avec largeur inconnue: %q", got)
	}
}
