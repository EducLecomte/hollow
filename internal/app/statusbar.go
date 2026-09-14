package app

import (
	"strings"
	"unicode/utf8"

	"github.com/rivo/tview"
)

// Modes de la barre d'état : chaque mode correspond à un contexte d'interface
// et détermine le jeu de raccourcis affichés en bas d'écran.
const (
	statusModeDefault = "default" // Panneau unique : explorateur + visualiseur
	statusModeDual    = "dual"    // Double panneau (local ou distant)
	statusModeArchive = "archive" // Navigation dans une archive
	statusModeEdit    = "edit"    // Écran éditeur (footer dédié)
	statusModeView    = "view"    // Visualiseur sous focus
)

// statusBarItem décrit un raccourci affiché dans la barre d'état.
type statusBarItem struct {
	key   string
	label string
}

// statusBarItems renvoie les raccourcis affichés pour le mode donné.
// L'ordre des tableaux est aussi la priorité d'affichage : sur un terminal
// étroit, les raccourcis sont coupés de la fin vers le début (les moins
// essentiels d'abord). L'aide F1 reste la référence complète.
func statusBarItems(mode string) []statusBarItem {
	switch mode {
	case statusModeDual:
		return []statusBarItem{
			{"F1", "Aide"},
			{"F2", "Tri"},
			{"F4", "Extraire"},
			{"F5", "Lien"},
			{"F6", "Droits"},
			{"F7", "Créer"},
			{"F8", "Copier"},
			{"F9", "Cachés"},
			{"Tab", "Panneau"},
			{"Suppr", "Effacer"},
			{"Esc", "Quitter"},
		}
	case statusModeArchive:
		return []statusBarItem{
			{"F1", "Aide"},
			{"F4", "Extraire"},
			{"Entrée", "Aperçu"},
		}
	case statusModeEdit:
		return []statusBarItem{
			{"F1", "Aide"},
			{"Ctrl+S", "Sauver"},
			{"Ctrl+F", "Chercher"},
			{"Ctrl+R", "Remplacer"},
			{"Ctrl+K/U", "C/V"},
			{"Ctrl+Z/Y", "Annuler/Refaire"},
			{"Esc", "Quitter"},
		}
	case statusModeView:
		return []statusBarItem{
			{"F1", "Aide"},
			{"Tab", "Retour"},
			{"Ctrl+X", "Quitter"},
			{"Flèches", "Défiler"},
		}
	default: // statusModeDefault
		return []statusBarItem{
			{"F1", "Aide"},
			{"F2", "Tri"},
			{"F3", "FTP"},
			{"F4", "Extraire"},
			{"F5", "Lien"},
			{"F6", "Droits"},
			{"F7", "Créer"},
			{"F8", "Double"},
			{"F9", "Cachés"},
			{"Tab", "Aperçu"},
			{"Suppr", "Effacer"},
		}
	}
}

// renderBindingsBar construit le texte coloré de la barre d'état pour le mode
// donné, en ne conservant que les raccourcis qui tiennent dans width colonnes.
// Si des raccourcis sont coupés, un « … » final l'indique.
//
// width <= 0 (avant le premier dessin) signifie « pas de troncature ».
// Tous les libellés sont des caractères étroits, compter les runes donne
// donc la largeur écran exacte.
func renderBindingsBar(mode string, width int) string {
	items := statusBarItems(mode)
	if width <= 0 {
		width = 1 << 30
	}

	var sb strings.Builder
	plain := 0
	dropped := false
	for i, it := range items {
		add := utf8.RuneCountInString(it.key + ": " + it.label)
		if i > 0 {
			add += 3 // séparateur " | "
		}
		if i > 0 && plain+add > width {
			dropped = true
			break
		}
		if i > 0 {
			sb.WriteString(" | ")
		}
		sb.WriteString("[yellow]" + it.key + ":[white] " + it.label)
		plain += add
	}
	if dropped {
		sb.WriteString(" …")
	}
	return sb.String()
}

// panelStatusMode renvoie le mode de la barre d'état quand un panneau est
// sous focus.
func (e *EditorApp) panelStatusMode() string {
	if e.IsDualPane() {
		return statusModeDual
	}
	if e.ActivePanel != nil && e.ActivePanel.IsArchive() {
		return statusModeArchive
	}
	return statusModeDefault
}

// statusModeFromFocus détermine le mode de la barre d'état à partir du
// composant actuellement focalisé.
func (e *EditorApp) statusModeFromFocus() string {
	focus := e.App.GetFocus()
	if focus == e.Viewer {
		return statusModeView
	}
	if _, ok := focus.(*tview.TextArea); ok {
		return statusModeEdit
	}
	return e.panelStatusMode()
}
