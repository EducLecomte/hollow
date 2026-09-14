package app

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
)

// setupExplorerHandlers configure les écouteurs d'événements pour les listes de fichiers.
func (e *EditorApp) setupExplorerHandlers() {
	e.setupPanelHandlers(e.LeftPanel)
	e.setupPanelHandlers(e.RightPanel)
}

// setupPanelHandlers configure les raccourcis spécifiques à chaque panneau.
func (e *EditorApp) setupPanelHandlers(p *PanelState) {
	p.List.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Shift+F6 pour copier en sens inverse lorsque le double panneau est actif
		if (event.Key() == tcell.KeyF6 && (event.Modifiers()&tcell.ModShift != 0)) || event.Key() == tcell.KeyF18 {
			if e.IsDualPane() {
				e.copySelectedBetweenPanels(true)
				return nil
			}
		}

		switch event.Key() {
		case tcell.KeyTab:
			if e.IsDualPane() {
				e.SwitchActivePanel()
			} else {
				// En mode simple, Tab passe au visualiseur
				e.App.SetFocus(e.Viewer)
			}
			return nil
		case tcell.KeyBacktab:
			if e.ShowFavs && p == e.LeftPanel {
				e.App.SetFocus(e.FavList)
			} else if e.IsDualPane() {
				e.SwitchActivePanel()
			} else {
				e.App.SetFocus(e.Viewer)
			}
			return nil
		case tcell.KeyEscape:
			if e.DualPaneMode {
				e.toggleDualPaneMode()
				return nil
			}
		case tcell.KeyF2:
			e.cycleSortKey()
			return nil
		case tcell.KeyF5:
			e.showSymlinkDialog()
			return nil
		case tcell.KeyF4:
			e.extractSelectedArchive()
			return nil
		case tcell.KeyF9:
			e.toggleHiddenFiles()
			return nil
		case tcell.KeyBackspace, tcell.KeyCtrlH:
			// La touche Ctrl+H envoie l'octet 0x08, que tcell décode
			// toujours en KeyBackspace (idempotent avec le Backspace physique).
			// Dans la liste de fichiers, le Backspace n'a pas d'autre rôle,
			// il sert donc (comme Ctrl+H) à basculer l'affichage des fichiers cachés.
			// Dans l'éditeur, KeyBackspace conserve son rôle natif (supprimer en arrière).
			e.toggleHiddenFiles()
			return nil
		case tcell.KeyCtrlO:
			e.toggleSortDirection()
			return nil
		case tcell.KeyF6:
			e.showChmodDialog()
			return nil
		case tcell.KeyCtrlT:
			e.toggleDualPaneMode()
			return nil
		case tcell.KeyCtrlR:
			e.showRenameDialog()
			return nil
		case tcell.KeyCtrlB:
			e.toggleFavorites()
			return nil
		case tcell.KeyCtrlD:
			item := p.GetSelectedItem()
			if item != nil && item.IsDir {
				e.addFavorite(filepath.Join(p.CurrentDir, item.Name))
			} else {
				e.updateStatusTemp("[red]Seuls les dossiers peuvent être mis en favoris")
			}
			return nil
		case tcell.KeyCtrlX:
			e.showQuitConfirmation()
			return nil
		case tcell.KeyF7:
			e.showNewElementDialog()
			return nil
		case tcell.KeyF8:
			if !e.IsDualPane() {
				e.toggleDualPaneMode()
			} else {
				e.copySelectedBetweenPanels(false)
			}
			return nil
		case tcell.KeyCtrlK:
			item := p.GetSelectedItem()
			if item != nil {
				e.prepareCopyFile(filepath.Join(p.CurrentDir, item.Name))
			}
			return nil
		case tcell.KeyCtrlU:
			e.pasteFile()
			return nil
		case tcell.KeyCtrlE:
			e.extractSelectedArchive()
			return nil
		case tcell.KeyDelete:
			e.showDeleteConfirmation()
			return nil
		}
		return event
	})
}
