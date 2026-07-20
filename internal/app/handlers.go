package app

import (
	"github.com/EducLecomte/go_hollow_project/internal/utils"
	"github.com/EducLecomte/go_hollow_project/internal/vfs"
	"github.com/gdamore/tcell/v2"
)

// setupHandlers configure les écouteurs d'événements clavier globaux pour l'application.
func (e *EditorApp) setupHandlers() {
	// 1. Raccourcis Globaux (Application)
	e.App.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		allowedCtrlKeys := map[tcell.Key]bool{
			tcell.KeyCtrlS: true, tcell.KeyCtrlF: true, tcell.KeyCtrlD: true,
			tcell.KeyCtrlK: true, tcell.KeyCtrlU: true, tcell.KeyCtrlB: true,
			tcell.KeyCtrlV: true, tcell.KeyCtrlX: true, tcell.KeyCtrlE: true,
			tcell.KeyCtrlP: true, tcell.KeyCtrlG: true, tcell.KeyCtrlT: true, tcell.KeyCtrlO: true,
			tcell.KeyCtrlA: true, tcell.KeyCtrlN: true, tcell.KeyCtrlR: true,
			tcell.KeyTab: true, tcell.KeyEnter: true,
			tcell.KeyBackspace: true, tcell.KeyBackspace2: true,
		}

		switch event.Key() {
		case tcell.KeyCtrlG:
			if e.Pages.HasPage("help") || e.Pages.HasPage("quit") ||
				e.Pages.HasPage("newfile") || e.Pages.HasPage("newdir") ||
				e.Pages.HasPage("delete") || e.Pages.HasPage("save_confirm") ||
				e.Pages.HasPage("ftp") {
				return event
			}

			// Détection du contexte pour l'aide
			helpContent := utils.HelpContentExplorer
			if _, ok := e.FileSystem.(*vfs.ArchiveFS); ok {
				helpContent = utils.HelpContentArchive
			}
			e.showHelp(helpContent)
			return nil
		case tcell.KeyCtrlT:
			if e.Pages.HasPage("help") || e.Pages.HasPage("quit") ||
				e.Pages.HasPage("newfile") || e.Pages.HasPage("newdir") ||
				e.Pages.HasPage("delete") || e.Pages.HasPage("ftp") {
				return event
			}
			e.showFTPDialog()
			return nil
		case tcell.KeyCtrlB:
			e.toggleFavorites()
			return nil
		case tcell.KeyCtrlP:
			e.showFuzzyFinder()
			return nil
		case tcell.KeyCtrlC:
			return nil
		}

		if event.Modifiers()&tcell.ModCtrl != 0 {
			if !allowedCtrlKeys[event.Key()] {
				return nil
			}
		}
		if event.Modifiers()&tcell.ModAlt != 0 {
			return nil
		}
		return event
	})

	e.setupExplorerHandlers()
	e.setupViewerHandlers()
}
