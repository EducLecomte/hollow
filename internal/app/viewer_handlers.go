package app

import (
	"github.com/EducLecomte/hollow/internal/utils"
	"github.com/EducLecomte/hollow/internal/vfs"
	"github.com/gdamore/tcell/v2"
)

// setupViewerHandlers gère les entrées clavier pour la zone de visualisation (lecture seule).
func (e *EditorApp) setupViewerHandlers() {
	e.Viewer.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		key := event.Key()

		// Aide contextuelle F1
		if key == tcell.KeyF1 {
			helpContent := utils.HelpContentExplorer
			if e.ActivePanel != nil {
				if _, ok := e.ActivePanel.FileSystem.(*vfs.ArchiveFS); ok {
					helpContent = utils.HelpContentArchive
				}
			}
			e.showHelp(helpContent)
			return nil
		}

		// Navigation vers l'explorateur ou les favoris
		if key == tcell.KeyTab {
			if e.ShowFavs {
				e.App.SetFocus(e.FavList)
			} else {
				e.App.SetFocus(e.LeftPanel.List)
			}
			return nil
		}
		if key == tcell.KeyBacktab {
			e.App.SetFocus(e.LeftPanel.List)
			return nil
		}
		if key == tcell.KeyF8 || key == tcell.KeyCtrlT {
			e.toggleDualPaneMode()
			return nil
		}
		if key == tcell.KeyCtrlX {
			e.showQuitConfirmation()
			return nil
		}

		return event
	})
}
