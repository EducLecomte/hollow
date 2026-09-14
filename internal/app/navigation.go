package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/EducLecomte/hollow/internal/utils"
	"github.com/EducLecomte/hollow/internal/vfs"
	"github.com/rivo/tview"
)

// refreshPanel recharge la liste des fichiers d'un panneau spécifique et met à jour ses affichages.
func (e *EditorApp) refreshPanel(p *PanelState) {
	if p == nil {
		return
	}

	go func() {
		files, err := p.FileSystem.List(context.Background(), p.CurrentDir)

		e.App.QueueUpdateDraw(func() {
			p.List.Clear()
			p.List.AddItem("..", "", 0, nil)

			if err != nil {
				p.CurrentFiles = nil
				p.UpdateInfoBox(nil)
				p.UpdateTitle(p == e.ActivePanel)
				if e.PathBar != nil && p == e.ActivePanel {
					e.PathBar.SetText(fmt.Sprintf(" Path: %s", utils.ShortenPath(p.CurrentDir)))
				}
				e.updateStatus(fmt.Sprintf("[red]Erreur listage (%s): %v", p.DisplayName(), err))
				return
			}

			// Masquage des fichiers cachés (commençant par '.') si désactivé
			if !e.ShowHidden {
				visible := files[:0]
				for _, f := range files {
					if !strings.HasPrefix(f.Name, ".") {
						visible = append(visible, f)
					}
				}
				files = visible
			}

			// Tri des entrées : dossiers d'abord, puis selon le critère et la direction choisis
			e.sortFiles(files)

			p.CurrentFiles = files
			p.UpdateTitle(p == e.ActivePanel)
			if e.PathBar != nil && p == e.ActivePanel {
				e.PathBar.SetText(fmt.Sprintf(" Path: %s", utils.ShortenPath(p.CurrentDir)))
			}

			selectedIndex := 0
			for i, f := range files {
				var displayName string
				if f.IsDir {
					displayName = "[#ff8c00]" + f.Name + "/"
				} else {
					displayName = f.Name
				}
				p.List.AddItem(displayName, "", 0, nil)

				if p.initialFileSelected != "" && f.Name == p.initialFileSelected {
					selectedIndex = i + 1
				}
			}

			if selectedIndex > 0 {
				p.List.SetCurrentItem(selectedIndex)
				p.initialFileSelected = ""
			}

			// Met à jour l'InfoBox pour la sélection
			curIdx := p.List.GetCurrentItem()
			if curIdx > 0 && curIdx-1 < len(p.CurrentFiles) {
				p.UpdateInfoBox(&p.CurrentFiles[curIdx-1])
			} else {
				p.UpdateInfoBox(nil)
			}

			// Si on est en mode par défaut avec visualiseur, on met à jour le visualiseur
			if p == e.LeftPanel && !e.IsDualPane() {
				e.triggerViewerPreviewForCurrentItem()
			}
		})
	}()
}

// refreshActivePanel rafraîchit le panneau actuellement sous focus.
func (e *EditorApp) refreshActivePanel() {
	e.refreshPanel(e.ActivePanel)
}

// refreshBothPanels recharge simultanément les deux panneaux.
func (e *EditorApp) refreshBothPanels() {
	e.refreshPanel(e.LeftPanel)
	if e.IsDualPane() {
		e.refreshPanel(e.RightPanel)
	}
}

// refreshFileList maintient la compatibilité pour rafraîchir le panneau actif.
func (e *EditorApp) refreshFileList() {
	e.refreshActivePanel()
}

// handleFileSelection traite l'action de validation sur un élément d'un panneau donné.
func (e *EditorApp) handleFileSelection(p *PanelState, index int) {
	if p == nil {
		p = e.ActivePanel
	}

	if index == 0 {
		if p.CurrentDir == "/" || p.CurrentDir == "." || p.CurrentDir == "" {
			// Si on est sur un serveur distant, remonter au-dessus de la racine équivaut à se déconnecter
			if p.IsRemote() {
				e.disconnectRemote(p)
				return
			}
			// Si on était dans une archive
			if p.PreviousFS != nil {
				if closer, ok := p.FileSystem.(io.Closer); ok {
					_ = closer.Close()
				}
				p.FileSystem = p.PreviousFS
				p.CurrentDir = p.PreviousDir
				p.PreviousFS = nil
				p.RemoteLabel = ""
				e.refreshPanel(p)
				e.updatePanelFocus()
				return
			}
		}

		p.CurrentDir = filepath.Dir(p.CurrentDir)
		e.refreshPanel(p)
		e.updatePanelFocus()
		return
	}

	if p.CurrentFiles == nil || index-1 >= len(p.CurrentFiles) {
		return
	}

	file := p.CurrentFiles[index-1]
	targetPath := filepath.Join(p.CurrentDir, file.Name)

	if file.IsDir {
		p.CurrentDir = targetPath
		e.refreshPanel(p)
	} else if utils.IsArchive(file.Name) {
		ctx, cancel := context.WithCancel(context.Background())
		e.showLoadingDialog("Chargement", fmt.Sprintf("Ouverture de %s en cours...", file.Name), cancel)

		go func() {
			archiveFS, err := vfs.NewArchiveFS(ctx, targetPath)

			e.App.QueueUpdateDraw(func() {
				e.removeLoadingPage()

				if err != nil {
					if err == context.Canceled {
						e.updateStatusTemp("[yellow]Ouverture annulée.")
					} else {
						e.updateStatusTemp(fmt.Sprintf("[red]Erreur d'ouverture d'archive: %v", err))
					}
					return
				}
				p.PreviousFS = p.FileSystem
				p.PreviousDir = p.CurrentDir
				p.FileSystem = archiveFS
				p.CurrentDir = "/"
				e.refreshPanel(p)
				e.statusMode = statusModeArchive
				e.updateStatusTemp(fmt.Sprintf("[green]Exploration de l'archive: %s", file.Name))
			})
		}()
	} else {
		e.openFile(targetPath, false)
	}
}

// openFile lit le contenu d'un fichier via le VFS du panneau actif de manière asynchrone et lance l'éditeur.
func (e *EditorApp) openFile(path string, force bool) {
	p := e.ActivePanel
	ctx, cancel := context.WithCancel(context.Background())
	e.showLoadingDialog("Chargement", fmt.Sprintf("Ouverture de %s...", filepath.Base(path)), cancel)

	go func() {
		reader, err := p.FileSystem.Read(ctx, path)
		if err != nil {
			e.App.QueueUpdateDraw(func() {
				e.removeLoadingPage()
				e.updateStatus(fmt.Sprintf("[red]Erreur lecture: %v", err))
			})
			return
		}
		defer reader.Close()

		buf := new(bytes.Buffer)
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

			if !force && buf.Len() > 0 && utils.IsBinary(buf.Bytes()) {
				e.App.QueueUpdateDraw(func() {
					e.removeLoadingPage()
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
					e.removeLoadingPage()
					e.updateStatus(fmt.Sprintf("[red]Erreur de lecture: %v", err))
				})
				return
			}
		}

		content := strings.ReplaceAll(buf.String(), "\r", "")
		e.App.QueueUpdateDraw(func() {
			e.removeLoadingPage()
			e.FilePath = path
			e.showFullEditor(content)
		})
	}()
}

// previewFile lit les premiers octets d'un fichier pour le visualiseur.
func (e *EditorApp) previewFile(ctx context.Context, fs vfs.VFS, path string) {
	reader, err := fs.Read(ctx, path)
	if err != nil {
		e.App.QueueUpdateDraw(func() {
			e.Viewer.SetText(fmt.Sprintf("[red]Erreur lecture: %v", err))
		})
		return
	}
	defer reader.Close()

	buf := new(bytes.Buffer)
	_, _ = io.CopyN(buf, reader, 10000)

	select {
	case <-ctx.Done():
		return
	default:
	}

	content := strings.ReplaceAll(buf.String(), "\r", "")

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
		e.Viewer.SetText(tview.TranslateANSI(highlighted))
		e.Viewer.ScrollToBeginning()
		e.Viewer.SetTitle(fmt.Sprintf(" Visualiseur: %s ", filepath.Base(path)))
	})
}

// previewDirectory génère une arborescence textuelle de manière asynchrone pour le visualiseur.
func (e *EditorApp) previewDirectory(ctx context.Context, fs vfs.VFS, path string) {
	files, err := fs.List(ctx, path)
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
