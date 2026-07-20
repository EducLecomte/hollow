package app

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
)

// setupExplorerHandlers gère les entrées clavier pour la liste de fichiers
func (e *EditorApp) setupExplorerHandlers() {
	e.FileList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			e.App.SetFocus(e.Viewer)
			return nil
		case tcell.KeyBacktab:
			if e.ShowFavs {
				e.App.SetFocus(e.FavList)
			} else {
				e.App.SetFocus(e.Viewer)
			}
			return nil
		case tcell.KeyCtrlB:
			e.toggleFavorites()
			return nil
		case tcell.KeyCtrlA:
			index := e.FileList.GetCurrentItem()
			if index > 0 && index-1 < len(e.CurrentFiles) {
				file := e.CurrentFiles[index-1]
				if file.IsDir {
					e.addFavorite(filepath.Join(e.CurrentDir, file.Name))
				} else {
					e.updateStatusTemp("[red]Seuls les dossiers peuvent être mis en favoris")
				}
			}
			return nil
		case tcell.KeyCtrlX:
			e.showQuitConfirmation()
			return nil
		case tcell.KeyCtrlF:
			e.showNewFileDialog()
			return nil
		case tcell.KeyCtrlD:
			e.showNewDirDialog()
			return nil
		case tcell.KeyCtrlK:
			index := e.FileList.GetCurrentItem()
			if index > 0 && index-1 < len(e.CurrentFiles) {
				file := e.CurrentFiles[index-1]
				e.prepareCopyFile(filepath.Join(e.CurrentDir, file.Name))
			}
			return nil
		case tcell.KeyCtrlU:
			e.pasteFile()
			return nil
		case tcell.KeyCtrlE:
			e.extractSelectedArchive()
			return nil
		case tcell.KeyDelete, tcell.KeyCtrlR:
			e.showDeleteConfirmation()
			return nil
		case tcell.KeyCtrlO:
			e.showChmodDialog()
			return nil
		}
		return event
	})
}


