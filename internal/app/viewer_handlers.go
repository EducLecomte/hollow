package app

import (
	"github.com/EducLecomte/go_hollow_project/internal/utils"
	"github.com/EducLecomte/go_hollow_project/internal/vfs"
	"github.com/gdamore/tcell/v2"
)

// setupViewerHandlers gère les entrées clavier pour la zone de visualisation (lecture seule)
func (e *EditorApp) setupViewerHandlers() {
	e.Viewer.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		key := event.Key()

		// Aide contextuelle Ctrl+G
		if key == tcell.KeyCtrlG {
			helpContent := utils.HelpContentExplorer
			if _, ok := e.FileSystem.(*vfs.ArchiveFS); ok {
				helpContent = utils.HelpContentArchive
			}
			e.showHelp(helpContent)
			return nil
		}

		// Navigation (Tab ou Ctrl+X) vers l'explorateur ou les favoris
		if key == tcell.KeyTab {
			if e.ShowFavs {
				e.App.SetFocus(e.FavList)
			} else {
				e.App.SetFocus(e.FileList)
			}
			return nil
		}
		if key == tcell.KeyBacktab {
			e.App.SetFocus(e.FileList)
			return nil
		}
		if key == tcell.KeyCtrlX {
			e.showQuitConfirmation()
			return nil
		}

		// Le TextView (Viewer) gère nativement les flèches pour le défilement
		return event
	})
}
