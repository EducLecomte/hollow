package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort" // Importé pour trier la liste de fichiers (dossiers d'abord, puis fichiers)
	"strings"

	"github.com/EducLecomte/go_hollow_project/internal/utils"
	"github.com/EducLecomte/go_hollow_project/internal/vfs"
	"github.com/rivo/tview"
)

// refreshFileList recharge la liste des fichiers du répertoire courant et met à jour l'affichage de l'explorateur.
func (e *EditorApp) refreshFileList() {
	go func() {
		files, err := e.FileSystem.List(context.Background(), e.CurrentDir)

		e.App.QueueUpdateDraw(func() {
			e.FileList.Clear()
			e.FileList.AddItem("..", "", 0, nil)

			if err != nil {
				e.CurrentFiles = nil
				e.updateStatus(fmt.Sprintf("[red]Erreur listage: %v", err))
				return
			}

			// Tri des entrées : dossiers d'abord, puis fichiers par ordre alphabétique (insensible à la casse)
			sort.Slice(files, func(i, j int) bool {
				if files[i].IsDir && !files[j].IsDir {
					return true
				}
				if !files[i].IsDir && files[j].IsDir {
					return false
				}
				return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
			})

			e.PathBar.SetText(fmt.Sprintf(" Path: %s", utils.ShortenPath(e.CurrentDir)))
			e.CurrentFiles = files

			// Index de sélection initialisé à 0 ("..")
			selectedIndex := 0

			for i, f := range files {
				var displayName string
				if f.IsDir {
					displayName = "[#ff8c00]" + f.Name + "/"
				} else {
					displayName = f.Name
				}
				e.FileList.AddItem(displayName, "", 0, nil)

				// Si un fichier initial est spécifié et correspond à l'élément courant,
				// on enregistre son index de liste (i + 1 car l'index 0 est "..")
				if e.initialFileSelected != "" && f.Name == e.initialFileSelected {
					selectedIndex = i + 1
				}
			}

			// Si un fichier a été identifié pour sélection, on applique le changement de focus
			if selectedIndex > 0 {
				e.FileList.SetCurrentItem(selectedIndex)
				// On vide le champ pour éviter de repositionner lors des prochains rafraîchissements
				e.initialFileSelected = ""
			}
		})
	}()
}

// handleFileSelection traite l'action de validation sur un élément de la liste (navigation, ouverture de fichier ou d'archive).
func (e *EditorApp) handleFileSelection(index int) {
	if index == 0 {
		if e.CurrentDir == "/" || e.CurrentDir == "." || e.CurrentDir == "" {
			if e.PreviousFS != nil {
				// Sortie du système de fichiers virtuel
				e.FileSystem = e.PreviousFS
				e.CurrentDir = e.PreviousDir
				e.PreviousFS = nil
				e.refreshFileList()
				e.updateStatus(utils.HelpMsgFiles)
				return
			}
		}

		e.CurrentDir = filepath.Dir(e.CurrentDir)
		e.refreshFileList()

		// Mise à jour de la barre d'état selon le contexte
		if _, ok := e.FileSystem.(*vfs.ArchiveFS); ok {
			e.updateStatus(utils.HelpMsgArchive)
		} else {
			e.updateStatus(utils.HelpMsgFiles)
		}
		return
	}

	if e.CurrentFiles == nil || index-1 >= len(e.CurrentFiles) {
		return
	}

	file := e.CurrentFiles[index-1]
	targetPath := filepath.Join(e.CurrentDir, file.Name)

	if file.IsDir {
		e.CurrentDir = targetPath
		e.refreshFileList()
	} else if utils.IsArchive(file.Name) {
		ctx, cancel := context.WithCancel(context.Background())
		e.showLoadingDialog("Chargement", fmt.Sprintf("Ouverture de %s en cours...", file.Name), cancel)

		go func() {
			archiveFS, err := vfs.NewArchiveFS(ctx, targetPath)

			e.App.QueueUpdateDraw(func() {
				e.Pages.RemovePage("loading")

				if err != nil {
					if err == context.Canceled {
						e.updateStatusTemp("[yellow]Ouverture annulée.")
					} else {
						e.updateStatusTemp(fmt.Sprintf("[red]Erreur d'ouverture d'archive: %v", err))
					}
					return
				}
				e.PreviousFS = e.FileSystem
				e.PreviousDir = e.CurrentDir
				e.FileSystem = archiveFS
				e.CurrentDir = "/"
				e.refreshFileList()
				e.updateStatus(utils.HelpMsgArchive) // Aide spécifique aux archives
				e.updateStatusTemp(fmt.Sprintf("[green]Exploration de l'archive: %s", file.Name))
			})
		}()
	} else {
		e.openFile(targetPath, false)
	}
}

// openFile lit le contenu d'un fichier via le VFS de manière asynchrone et lance l'éditeur.
// Le paramètre force permet de passer outre la détection de fichier binaire.
func (e *EditorApp) openFile(path string, force bool) {
	ctx, cancel := context.WithCancel(context.Background())
	e.showLoadingDialog("Chargement", fmt.Sprintf("Ouverture de %s...", filepath.Base(path)), cancel)

	go func() {
		reader, err := e.FileSystem.Read(ctx, path)
		if err != nil {
			e.App.QueueUpdateDraw(func() {
				e.Pages.RemovePage("loading")
				e.updateStatus(fmt.Sprintf("[red]Erreur lecture: %v", err))
			})
			return
		}
		defer reader.Close()

		buf := new(bytes.Buffer)
		// Lecture par blocs pour permettre l'annulation
		tempBuf := make([]byte, 32*1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			n, err := reader.Read(tempBuf)
			if n > 0 {
				buf.Write(tempBuf[:n])
			}

			// Détection binaire sur le premier bloc lu (si pas forcé)
			if !force && buf.Len() > 0 && utils.IsBinary(buf.Bytes()) {
				e.App.QueueUpdateDraw(func() {
					e.Pages.RemovePage("loading")
					e.showBinaryOpenConfirmation(path, func() {
						e.openFile(path, true)
					})
				})
				return
			}

			if err == io.EOF {
				break
			}
			if err != nil {
				e.App.QueueUpdateDraw(func() {
					e.Pages.RemovePage("loading")
					e.updateStatus(fmt.Sprintf("[red]Erreur de lecture: %v", err))
				})
				return
			}
		}

		content := strings.ReplaceAll(buf.String(), "\r", "")
		e.App.QueueUpdateDraw(func() {
			e.Pages.RemovePage("loading")
			e.FilePath = path
			e.showFullEditor(content)
		})
	}()
}

// previewFile lit les premiers octets d'un fichier de manière asynchrone pour le visualiseur.
func (e *EditorApp) previewFile(ctx context.Context, path string) {
	reader, err := e.FileSystem.Read(ctx, path)
	if err != nil {
		e.App.QueueUpdateDraw(func() {
			e.Viewer.SetText(fmt.Sprintf("[red]Erreur lecture: %v", err))
		})
		return
	}
	defer reader.Close()

	buf := new(bytes.Buffer)
	// Lecture limitée (10 Ko)
	_, _ = io.CopyN(buf, reader, 10000)

	select {
	case <-ctx.Done():
		return
	default:
	}

	content := strings.ReplaceAll(buf.String(), "\r", "")

	// Détection des fichiers binaires pour éviter les gels ou affichages illisibles
	if utils.IsBinary(buf.Bytes()) {
		e.App.QueueUpdateDraw(func() {
			fileName := filepath.Base(path)
			fileDesc := utils.GetBinaryFileDescription(fileName)
			e.Viewer.SetText(fmt.Sprintf("\n\n  [red][ Fichier identifié comme %s - Aperçu désactivé ]", fileDesc))
			e.Viewer.SetTitle(fmt.Sprintf(" Visualiseur: %s ", fileName))
		})
		return
	}

	highlighted := utils.Highlight(content, path)

	e.App.QueueUpdateDraw(func() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// On utilise TranslateANSI pour supporter la coloration de Chroma via tview
		e.Viewer.SetText(tview.TranslateANSI(highlighted))
		e.Viewer.ScrollToBeginning()
		e.Viewer.SetTitle(fmt.Sprintf(" Visualiseur: %s ", filepath.Base(path)))
	})
}

// previewDirectory génère une arborescence textuelle de manière asynchrone pour le visualiseur.
func (e *EditorApp) previewDirectory(ctx context.Context, path string) {
	files, err := e.FileSystem.List(ctx, path)
	if err != nil {
		e.App.QueueUpdateDraw(func() {
			e.Viewer.SetText(fmt.Sprintf("[red]Erreur lecture dossier: %v", err))
		})
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Contenu de [yellow]%s[white] :\n\n", filepath.Base(path)))

	if len(files) == 0 {
		sb.WriteString("  [gray](Dossier vide)")
	} else {
		for i, f := range files {
			connector := "├── "
			if i == len(files)-1 {
				connector = "└── "
			}
			if f.IsDir {
				sb.WriteString(fmt.Sprintf("%s[darkorange]%s/[white]\n", connector, f.Name))
			} else {
				sb.WriteString(fmt.Sprintf("%s%s\n", connector, f.Name))
			}
		}
	}

	e.App.QueueUpdateDraw(func() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		e.Viewer.SetText(sb.String())
		e.Viewer.ScrollToBeginning()
		e.Viewer.SetTitle(fmt.Sprintf(" Visualiseur: %s ", filepath.Base(path)))
	})
}
