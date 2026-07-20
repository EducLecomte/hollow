package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/EducLecomte/go_hollow_project/internal/utils"
	"github.com/EducLecomte/go_hollow_project/internal/vfs"
)

// createFile crée un nouveau fichier vide dans le répertoire courant et l'ouvre dans l'éditeur.
func (e *EditorApp) createFile(name string) {
	path := filepath.Join(e.CurrentDir, name)
	// On écrit un fichier vide
	err := e.FileSystem.Write(context.Background(), path, strings.NewReader(""))
	if err != nil {
		e.updateStatus(fmt.Sprintf("[red]Erreur création: %v", err))
		return
	}
	e.refreshFileList()
	e.openFile(path, true)
	e.updateStatus(fmt.Sprintf("[green]Fichier créé: %s", name))
}

// createDir crée un nouveau répertoire dans le répertoire courant.
func (e *EditorApp) createDir(name string) {
	path := filepath.Join(e.CurrentDir, name)
	err := e.FileSystem.Mkdir(context.Background(), path)
	if err != nil {
		e.updateStatus(fmt.Sprintf("[red]Erreur dossier: %v", err))
		return
	}
	e.refreshFileList()
	e.updateStatus(fmt.Sprintf("[green]Dossier créé: %s", name))
}

// prepareCopyFile mémorise le chemin de l'élément à copier pour une action de collage ultérieure.
func (e *EditorApp) prepareCopyFile(path string) {
	e.CopiedPath = path
	e.updateStatusTemp(fmt.Sprintf("Élément prêt à copier: %s", filepath.Base(path)))
}

// pasteFile copie l'élément précédemment mémorisé dans le répertoire courant, en gérant les doublons de noms.
func (e *EditorApp) pasteFile() {
	if e.CopiedPath == "" {
		e.updateStatusTemp("[red]Rien à coller")
		return
	}

	baseName := filepath.Base(e.CopiedPath)
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)

	finalName := baseName
	counter := 1

	for {
		exists := false
		for _, f := range e.CurrentFiles {
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

	dst := filepath.Join(e.CurrentDir, finalName)

	err := e.FileSystem.Copy(context.Background(), e.CopiedPath, dst)
	if err != nil {
		e.updateStatusTemp(fmt.Sprintf("[red]Erreur collage: %v", err))
		return
	}

	e.refreshFileList()
	e.updateStatusTemp(fmt.Sprintf("[green]Élément collé: %s", finalName))
}

// deleteElement supprime définitivement l'élément (fichier ou dossier) situé au chemin indiqué.
func (e *EditorApp) deleteElement(path string) {
	err := e.FileSystem.Remove(context.Background(), path)
	if err != nil {
		e.updateStatus(fmt.Sprintf("[red]Erreur suppression: %v", err))
		return
	}
	e.refreshFileList()
	e.updateStatus(fmt.Sprintf("[green]Supprimé: %s", filepath.Base(path)))
}

// saveLastDir persiste le chemin du répertoire courant dans un fichier temporaire pour permettre la synchronisation du shell à la fermeture.
func (e *EditorApp) saveLastDir() {
	path := fmt.Sprintf("/tmp/hollow_cwd_%s", os.Getenv("USER"))
	_ = os.WriteFile(path, []byte(e.CurrentDir), 0644)
}

// extractSelectedArchive gère l'extraction d'une archive complète ou d'un fichier spécifique vers le système de fichiers local.
func (e *EditorApp) extractSelectedArchive() {
	index := e.FileList.GetCurrentItem()
	if index <= 0 || index-1 >= len(e.CurrentFiles) {
		return
	}
	file := e.CurrentFiles[index-1]

	var srcFS vfs.VFS
	var dstFS vfs.VFS
	var srcPath string
	var destPath string
	var destName string

	// Détection du mode : sommes-nous DANS une archive ou en train d'en sélectionner une sur le disque ?
	archiveFS, isInside := e.FileSystem.(*vfs.ArchiveFS)

	if isInside {
		// MODE INDIVIDUEL : On extrait l'élément sélectionné vers le dossier hôte de l'archive
		srcFS = e.FileSystem
		dstFS = e.PreviousFS
		srcPath = filepath.Join(e.CurrentDir, file.Name)

		hostDir := filepath.Dir(archiveFS.ArchivePath)
		destName = file.Name
		destPath = filepath.Join(hostDir, destName)
	} else {
		// MODE COMPLET : On extrait l'archive sélectionnée vers un dossier "_extracted"
		if !utils.IsArchive(file.Name) {
			e.updateStatusTemp("[red]L'élément sélectionné n'est pas une archive")
			return
		}

		archivePath := filepath.Join(e.CurrentDir, file.Name)

		// Calcul du dossier de destination
		ext := filepath.Ext(file.Name)
		if strings.HasSuffix(strings.ToLower(file.Name), ".tar.gz") {
			destName = strings.TrimSuffix(file.Name, ".tar.gz")
		} else if strings.HasSuffix(strings.ToLower(file.Name), ".tgz") {
			destName = strings.TrimSuffix(file.Name, ".tgz")
		} else {
			destName = strings.TrimSuffix(file.Name, ext)
		}
		destName += "_extracted"
		destPath = filepath.Join(e.CurrentDir, destName)

		// Création d'un FS temporaire pour lire l'archive
		ctxTemp, cancelTemp := context.WithCancel(context.Background())
		tempFS, err := vfs.NewArchiveFS(ctxTemp, archivePath)
		if err != nil {
			cancelTemp()
			e.updateStatusTemp(fmt.Sprintf("[red]Erreur ouverture archive: %v", err))
			return
		}
		// On utilisera ce FS pour l'extraction
		srcFS = tempFS
		dstFS = e.FileSystem // On est sur le LocalFS
		srcPath = "/"
		defer cancelTemp()
		defer tempFS.Close()
	}

	ctx, cancel := context.WithCancel(context.Background())
	e.showLoadingDialog("Extraction", fmt.Sprintf("Extraction de %s...", file.Name), cancel)

	go func() {
		err := vfs.CopyRecursiveBetweenVFS(ctx, srcFS, dstFS, srcPath, destPath)

		e.App.QueueUpdateDraw(func() {
			e.Pages.RemovePage("loading")
			if err != nil {
				if err == context.Canceled {
					e.updateStatusTemp("[yellow]Extraction annulée.")
				} else {
					e.updateStatusTemp(fmt.Sprintf("[red]Erreur extraction: %v", err))
				}
			} else {
				if !isInside {
					e.refreshFileList()
				}
				e.updateStatusTemp(fmt.Sprintf("[green]Extraction réussie: %s", destName))
			}
		})
	}()
}
