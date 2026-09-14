package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EducLecomte/hollow/internal/utils"
	"github.com/EducLecomte/hollow/internal/vfs"
)

// createFile crée un nouveau fichier vide dans le répertoire du panneau actif et l'ouvre dans l'éditeur.
func (e *EditorApp) createFile(name string) {
	p := e.ActivePanel
	path := filepath.Join(p.CurrentDir, name)

	err := p.FileSystem.Write(context.Background(), path, strings.NewReader(""))
	if err != nil {
		e.updateStatus(fmt.Sprintf("[red]Erreur création: %v", err))
		return
	}
	e.refreshPanel(p)
	e.openFile(path, true)
	e.updateStatus(fmt.Sprintf("[green]Fichier créé: %s", name))
}

// createDir crée un nouveau répertoire dans le répertoire du panneau actif.
func (e *EditorApp) createDir(name string) {
	p := e.ActivePanel
	path := filepath.Join(p.CurrentDir, name)

	err := p.FileSystem.Mkdir(context.Background(), path)
	if err != nil {
		e.updateStatus(fmt.Sprintf("[red]Erreur dossier: %v", err))
		return
	}
	e.refreshPanel(p)
	e.updateStatus(fmt.Sprintf("[green]Dossier créé: %s", name))
}

// renameElement renomme un fichier ou un dossier du panneau actif.
func (e *EditorApp) renameElement(oldPath, newName string) {
	p := e.ActivePanel
	newPath := filepath.Join(p.CurrentDir, newName)
	if oldPath == newPath {
		e.updateStatusTemp("[yellow]Le nom est inchangé")
		return
	}

	if err := p.FileSystem.Rename(context.Background(), oldPath, newPath); err != nil {
		e.updateStatusTemp(fmt.Sprintf("[red]Erreur renommage: %v", err))
		return
	}
	e.refreshPanel(p)
	e.updateStatusTemp(fmt.Sprintf("[green]Élément renommé: %s", newName))
}

// createSymbolicLink crée un lien symbolique dans le répertoire du panneau actif.
func (e *EditorApp) createSymbolicLink(target, linkName string) {
	p := e.ActivePanel
	linkPath := filepath.Join(p.CurrentDir, linkName)
	if err := p.FileSystem.Symlink(context.Background(), target, linkPath); err != nil {
		e.updateStatusTemp(fmt.Sprintf("[red]Erreur lien symbolique: %v", err))
		return
	}
	e.refreshPanel(p)
	e.updateStatusTemp(fmt.Sprintf("[green]Lien symbolique créé: %s", linkName))
}

// prepareCopyFile mémorise le chemin et le système de fichiers pour une action de collage ultérieure.
func (e *EditorApp) prepareCopyFile(path string) {
	e.CopiedPath = path
	e.CopiedFS = e.ActivePanel.FileSystem
	e.updateStatusTemp(fmt.Sprintf("Élément prêt à copier: %s", filepath.Base(path)))
}

// pasteFile copie l'élément mémorisé dans le répertoire du panneau actif en gérant les doublons et les VFS différents.
func (e *EditorApp) pasteFile() {
	if e.CopiedPath == "" {
		e.updateStatusTemp("[red]Rien à coller")
		return
	}

	p := e.ActivePanel
	baseName := filepath.Base(e.CopiedPath)
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)

	finalName := baseName
	counter := 1

	for {
		exists := false
		for _, f := range p.CurrentFiles {
			if f.Name == finalName {
				exists = true
				break
			}
		}
		if !exists {
			break
		}
		if counter == 1 {
			finalName = fmt.Sprintf("%s_copy%s", nameWithoutExt, ext)
		} else {
			finalName = fmt.Sprintf("%s_copy%d%s", nameWithoutExt, counter, ext)
		}
		counter++
	}

	dst := filepath.Join(p.CurrentDir, finalName)

	var err error
	if e.CopiedFS != nil && e.CopiedFS != p.FileSystem {
		err = vfs.CopyRecursiveBetweenVFS(context.Background(), e.CopiedFS, p.FileSystem, e.CopiedPath, dst)
	} else {
		err = p.FileSystem.Copy(context.Background(), e.CopiedPath, dst)
	}

	if err != nil {
		e.updateStatusTemp(fmt.Sprintf("[red]Erreur collage: %v", err))
		return
	}

	e.refreshPanel(p)
	e.updateStatusTemp(fmt.Sprintf("[green]Élément collé: %s", finalName))
}

// deleteElement supprime définitivement l'élément situé au chemin indiqué sur le panneau actif.
func (e *EditorApp) deleteElement(path string) {
	p := e.ActivePanel
	err := p.FileSystem.Remove(context.Background(), path)
	if err != nil {
		e.updateStatus(fmt.Sprintf("[red]Erreur suppression: %v", err))
		return
	}
	e.refreshPanel(p)
	e.updateStatus(fmt.Sprintf("[green]Supprimé: %s", filepath.Base(path)))
}

// saveLastDir persiste le chemin du répertoire courant local pour synchroniser le shell à la fermeture.
func (e *EditorApp) saveLastDir() {
	path := fmt.Sprintf("/tmp/hollow_cwd_%s", os.Getenv("USER"))
	dir := e.ActivePanel.CurrentDir
	if e.ActivePanel.IsRemote() {
		dir = e.LeftPanel.CurrentDir
	}
	_ = os.WriteFile(path, []byte(dir), 0644)
}

// copySelectedBetweenPanels copie l'élément sélectionné vers l'autre panneau (F6 / Shift+F6).
func (e *EditorApp) copySelectedBetweenPanels(reverse bool) {
	srcPanel := e.ActivePanel
	dstPanel := e.InactivePanel()
	if reverse {
		srcPanel, dstPanel = dstPanel, srcPanel
	}

	item := srcPanel.GetSelectedItem()
	if item == nil || item.Name == ".." {
		e.updateStatusTemp("[yellow]Sélectionnez un fichier ou dossier valide à copier")
		return
	}

	srcPath := filepath.Join(srcPanel.CurrentDir, item.Name)
	dstPath := filepath.Join(dstPanel.CurrentDir, item.Name)

	if srcPanel.FileSystem == dstPanel.FileSystem && srcPanel.CurrentDir == dstPanel.CurrentDir {
		e.updateStatusTemp("[red]La source et la destination sont identiques")
		return
	}

	// Vérification préalable de l'existence de la destination
	go func() {
		ctxCheck, cancelCheck := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelCheck()
		_, err := dstPanel.FileSystem.Stat(ctxCheck, dstPath)
		exists := (err == nil)

		e.App.QueueUpdateDraw(func() {
			if exists {
				e.showOverwriteConfirmation(item.Name, dstPanel.DisplayName(), func(overwrite bool) {
					if overwrite {
						e.executeCopyBetweenPanels(srcPanel, dstPanel, srcPath, dstPath, item.Name)
					}
				})
			} else {
				e.executeCopyBetweenPanels(srcPanel, dstPanel, srcPath, dstPath, item.Name)
			}
		})
	}()
}

// executeCopyBetweenPanels lance la copie asynchrone entre deux panneaux avec dialogue
// d'attente, affichage de l'avancement en temps réel et bouton Annuler.
func (e *EditorApp) executeCopyBetweenPanels(srcPanel, dstPanel *PanelState, srcPath, dstPath, itemName string) {
	ctx, cancel := context.WithCancel(context.Background())
	header := fmt.Sprintf("Copie de : %s\nDe : [%s] %s\nVers : [%s] %s",
		itemName, srcPanel.DisplayName(), srcPanel.CurrentDir, dstPanel.DisplayName(), dstPanel.CurrentDir)
	e.showLoadingDialog("Copie en cours", header+"\n\n...", cancel)

	state := &progressState{}
	e.startProgressTicker(ctx, state, header)

	go func() {
		defer cancel()
		err := copyTreeWithProgress(ctx, srcPanel.FileSystem, dstPanel.FileSystem, srcPath, dstPath, state)

		e.App.QueueUpdateDraw(func() {
			e.removeLoadingPage()
			if err != nil {
				if err == context.Canceled {
					e.updateStatusTemp("[yellow]Copie annulée.")
				} else {
					e.updateStatusTemp(fmt.Sprintf("[red]Erreur copie: %v", err))
				}
			} else {
				e.updateStatusTemp(fmt.Sprintf("[green]Copie réussie: %s", itemName))
				// Rafraîchissement des deux panneaux
				e.refreshPanel(srcPanel)
				e.refreshPanel(dstPanel)
			}
		})
	}()
}

// extractSelectedArchive extrait une archive vers le répertoire du panneau opposé.
func (e *EditorApp) extractSelectedArchive() {
	srcPanel := e.ActivePanel
	dstPanel := e.InactivePanel()

	item := srcPanel.GetSelectedItem()
	if item == nil || item.Name == ".." {
		return
	}

	var srcFS vfs.VFS
	var srcPath string
	var destPath string
	var destName string
	var tempFSToClose vfs.VFS

	_, isInside := srcPanel.FileSystem.(*vfs.ArchiveFS)

	if isInside {
		// MODE INDIVIDUEL : on extrait l'élément sélectionné vers le panneau opposé
		srcFS = srcPanel.FileSystem
		srcPath = filepath.Join(srcPanel.CurrentDir, item.Name)
		destName = item.Name
		destPath = filepath.Join(dstPanel.CurrentDir, destName)
	} else {
		// MODE COMPLET : on extrait l'archive sélectionnée vers un dossier dans le panneau opposé
		if !utils.IsArchive(item.Name) {
			e.updateStatusTemp("[red]L'élément sélectionné n'est pas une archive")
			return
		}

		archivePath := filepath.Join(srcPanel.CurrentDir, item.Name)

		ext := filepath.Ext(item.Name)
		if strings.HasSuffix(strings.ToLower(item.Name), ".tar.gz") {
			destName = strings.TrimSuffix(item.Name, ".tar.gz")
		} else if strings.HasSuffix(strings.ToLower(item.Name), ".tgz") {
			destName = strings.TrimSuffix(item.Name, ".tgz")
		} else {
			destName = strings.TrimSuffix(item.Name, ext)
		}
		destName += "_extracted"
		destPath = filepath.Join(dstPanel.CurrentDir, destName)

		ctxTemp, cancelTemp := context.WithCancel(context.Background())
		tempFS, err := vfs.NewArchiveFS(ctxTemp, archivePath)
		if err != nil {
			cancelTemp()
			e.updateStatusTemp(fmt.Sprintf("[red]Erreur ouverture archive: %v", err))
			return
		}
		srcFS = tempFS
		srcPath = "/"
		tempFSToClose = tempFS
		defer cancelTemp()
	}

	ctx, cancel := context.WithCancel(context.Background())
	header := fmt.Sprintf("Extraction de %s vers [%s]...", item.Name, dstPanel.DisplayName())
	e.showLoadingDialog("Extraction", header+"\n\n...", cancel)

	state := &progressState{}
	e.startProgressTicker(ctx, state, header)

	go func() {
		defer cancel()
		if tempFSToClose != nil {
			defer tempFSToClose.Close()
		}

		err := copyTreeWithProgress(ctx, srcFS, dstPanel.FileSystem, srcPath, destPath, state)

		e.App.QueueUpdateDraw(func() {
			e.removeLoadingPage()
			if err != nil {
				if err == context.Canceled {
					e.updateStatusTemp("[yellow]Extraction annulée.")
				} else {
					e.updateStatusTemp(fmt.Sprintf("[red]Erreur extraction: %v", err))
				}
			} else {
				e.refreshPanel(dstPanel)
				e.updateStatusTemp(fmt.Sprintf("[green]Extraction réussie vers %s: %s", dstPanel.DisplayName(), destName))
			}
		})
	}()
}
