package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/EducLecomte/hollow/internal/utils"
	"github.com/EducLecomte/hollow/internal/vfs"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type EditorApp struct {
	// Infrastructure Tview
	App   *tview.Application
	Pages *tview.Pages

	// Panneaux
	LeftPanel   *PanelState // Panneau local principal
	RightPanel  *PanelState // Panneau distant (FTP/SFTP) ou secondaire en double panneau
	ActivePanel *PanelState // Panneau sous focus

	// Mode d'affichage
	DualPaneMode bool // Mode double panneau local activé manuellement

	// Composants de l'interface
	PathBar *tview.TextView // Barre de chemin supérieure
	Viewer  *tview.TextView // Visualiseur de fichier par défaut
	Status  *tview.TextView // Barre d'état inférieure
	FavList *tview.List     // Barre latérale des favoris

	// État de l'éditeur et Presse-papiers
	FilePath      string
	CopiedPath    string
	CopiedFS      vfs.VFS
	Clipboard     string
	LastSearch    string
	LastSearchPos int

	// Dossiers favoris
	Favorites []Favorite
	ShowFavs  bool

	// Tri et affichage des fichiers
	SortKey    int  // Critère de tri courant (voir display.go)
	SortAsc    bool // Direction du tri (true = croissant)
	ShowHidden bool // Afficher les fichiers cachés (commençant par '.')

	// Barre d'état (raccourcis contextuels, voir statusbar.go)
	statusMode     string
	statusOverride string          // Message personnalisé, "" = afficher les raccourcis de statusMode
	statusWidth    int             // Dernière largeur de terminal connue
	editorFooter   *tview.TextView // Footer de l'écran éditeur

	// Modale de chargement courante (transferts, etc.)
	loadingModal *tview.Modal

	// Gestion de l'asynchronisme
	previewCancel context.CancelFunc
}

// NewEditorApp initialise une nouvelle instance de l'application Hollow.
func NewEditorApp(initialPath string) *EditorApp {
	wd, err := filepath.Abs(".")
	if err != nil {
		wd = "/"
	}

	var fileToOpen string
	var fileSelected string

	if initialPath != "" {
		absPath, err := filepath.Abs(initialPath)
		if err == nil {
			info, err := os.Stat(absPath)
			if err == nil {
				if info.IsDir() {
					wd = absPath
				} else {
					wd = filepath.Dir(absPath)
					fileToOpen = absPath
					fileSelected = filepath.Base(absPath)
				}
			} else {
				wd = filepath.Dir(absPath)
				fileToOpen = absPath
				fileSelected = filepath.Base(absPath)
			}
		}
	}

	leftPanel := NewPanelState("left", wd, &vfs.LocalFS{})
	rightPanel := NewPanelState("right", wd, &vfs.LocalFS{})
	leftPanel.initialFileSelected = fileSelected

	e := &EditorApp{
		App:          tview.NewApplication(),
		Pages:        tview.NewPages(),
		LeftPanel:    leftPanel,
		RightPanel:   rightPanel,
		ActivePanel:  leftPanel,
		PathBar:      tview.NewTextView(),
		Viewer:       tview.NewTextView(),
		Status:       tview.NewTextView(),
		FavList:      tview.NewList(),
		DualPaneMode: false,
		SortAsc:      true,
		ShowHidden:   false,
	}

	e.loadFavorites()
	e.setupUI()
	e.setupFavHandlers()
	e.setupHandlers()

	e.refreshPanel(e.LeftPanel)

	if fileToOpen != "" {
		e.openFile(fileToOpen, false)
	}

	return e
}

// IsDualPane indique si l'interface doit afficher le double panneau
// (connecté à un serveur distant FTP/SFTP ou en mode double panneau manuel).
func (e *EditorApp) IsDualPane() bool {
	return e.DualPaneMode || (e.RightPanel != nil && e.RightPanel.IsRemote()) || (e.LeftPanel != nil && e.LeftPanel.IsRemote())
}

// toggleDualPaneMode active ou désactive uniquement le mode d'affichage double panneau.
func (e *EditorApp) toggleDualPaneMode() {
	e.DualPaneMode = !e.DualPaneMode
	if e.DualPaneMode {
		if e.RightPanel.CurrentDir == "" {
			e.RightPanel.CurrentDir = e.LeftPanel.CurrentDir
		}
		e.refreshPanel(e.RightPanel)
		e.rebuildMainLayout()
		e.updatePanelFocus()
		e.updateStatusTemp("[green]Mode double panneau activé (F6: copier | Tab: basculer | Esc: fermer)")
	} else {
		e.ActivePanel = e.LeftPanel
		e.rebuildMainLayout()
		e.updatePanelFocus()
		e.App.SetFocus(e.LeftPanel.List)
		e.triggerViewerPreviewForCurrentItem()
		e.updateStatusTemp("[yellow]Mode double panneau désactivé (visualiseur restauré)")
	}
}

// InactivePanel renvoie le panneau opposé au panneau actif.
func (e *EditorApp) InactivePanel() *PanelState {
	if e.ActivePanel == e.LeftPanel {
		return e.RightPanel
	}
	return e.LeftPanel
}

// SwitchActivePanel bascule le focus entre le panneau gauche et le panneau droit en mode double panneau.
func (e *EditorApp) SwitchActivePanel() {
	if !e.IsDualPane() {
		return
	}

	if e.ActivePanel == e.LeftPanel {
		e.ActivePanel = e.RightPanel
	} else {
		e.ActivePanel = e.LeftPanel
	}

	e.App.SetFocus(e.ActivePanel.List)
	e.updatePanelFocus()
}

// updatePanelFocus met à jour les bordures et titres des panneaux ainsi que la barre d'état.
func (e *EditorApp) updatePanelFocus() {
	e.LeftPanel.UpdateTitle(e.ActivePanel == e.LeftPanel)
	e.RightPanel.UpdateTitle(e.ActivePanel == e.RightPanel)

	e.statusOverride = ""
	e.statusMode = e.panelStatusMode()
	e.Status.SetText(renderBindingsBar(e.statusMode, e.statusWidth))
}

// setupPanelUI configure les comportements d'un panneau.
func (e *EditorApp) setupPanelUI(p *PanelState) {
	p.List.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		e.handleFileSelection(p, index)
	})

	p.List.SetFocusFunc(func() {
		e.ActivePanel = p
		e.updatePanelFocus()
		if e.PathBar != nil {
			e.PathBar.SetText(fmt.Sprintf(" Path: %s", utils.ShortenPath(p.CurrentDir)))
		}
	})

	isUpdatingList := false
	p.List.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index == 0 {
			p.UpdateInfoBox(nil)
			if !e.IsDualPane() {
				e.Viewer.SetText("").SetTitle(" Visualiseur ")
			}
			return
		}

		if p.CurrentFiles == nil || index-1 >= len(p.CurrentFiles) {
			return
		}

		file := p.CurrentFiles[index-1]
		p.UpdateInfoBox(&file)

		// En mode par défaut, prévisualisation en direct dans le visualiseur !
		if !e.IsDualPane() {
			e.triggerViewerPreview(p, file)
		}

		if isUpdatingList {
			return
		}
		isUpdatingList = true
		defer func() { isUpdatingList = false }()
		p.RefreshStyle(index)
	})
}

// triggerViewerPreview lance la prévisualisation asynchrone dans le visualiseur (mode par défaut).
func (e *EditorApp) triggerViewerPreview(p *PanelState, file vfs.FileInfo) {
	if e.previewCancel != nil {
		e.previewCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	e.previewCancel = cancel

	path := filepath.Join(p.CurrentDir, file.Name)

	go func() {
		time.Sleep(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			return
		default:
		}

		if file.IsDir {
			e.previewDirectory(ctx, p.FileSystem, path)
		} else {
			e.previewFile(ctx, p.FileSystem, path)
		}
	}()
}

// triggerViewerPreviewForCurrentItem rafraîchit le visualiseur avec l'élément courant du panneau gauche.
func (e *EditorApp) triggerViewerPreviewForCurrentItem() {
	if e.LeftPanel == nil || e.IsDualPane() {
		return
	}
	item := e.LeftPanel.GetSelectedItem()
	if item == nil {
		e.Viewer.SetText("").SetTitle(" Visualiseur ")
		return
	}
	e.triggerViewerPreview(e.LeftPanel, *item)
}

// setupUI configure la disposition des composants et styles.
func (e *EditorApp) setupUI() {
	e.PathBar.SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetTextColor(tcell.ColorBlack).
		SetBackgroundColor(tcell.ColorGreen)

	e.setupPanelUI(e.LeftPanel)
	e.setupPanelUI(e.RightPanel)

	// Visualiseur de fichier par défaut
	e.Viewer.SetBorder(true).SetTitle(" Visualiseur ").SetBorderColor(tcell.ColorWhite)
	e.Viewer.SetDynamicColors(true).SetRegions(true)
	e.Viewer.SetWrap(true)
	e.Viewer.SetFocusFunc(func() {
		e.Viewer.SetBorderColor(tcell.ColorYellow)
		e.statusOverride = ""
		e.statusMode = statusModeView
		e.Status.SetText(renderBindingsBar(e.statusMode, e.statusWidth))
	})
	e.Viewer.SetBlurFunc(func() {
		e.Viewer.SetBorderColor(tcell.ColorWhite)
	})

	// Barre d'état : raccourcis contextuels, adaptés à la largeur du terminal.
	e.Status.SetDynamicColors(true).SetTextAlign(tview.AlignCenter)

	// À chaque redimensionnement, on reconstruit la barre pour que les
	// raccourcis affichés tiennent toujours dans la largeur du terminal.
	e.App.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		w, _ := screen.Size()
		if w != e.statusWidth {
			e.statusWidth = w
			if e.statusOverride == "" {
				e.Status.SetText(renderBindingsBar(e.statusMode, w))
			}
			if e.editorFooter != nil {
				e.editorFooter.SetText(renderBindingsBar(statusModeEdit, w))
			}
		}
		return false
	})

	e.statusMode = e.statusModeFromFocus()
	e.Status.SetText(renderBindingsBar(e.statusMode, e.statusWidth))

	// Barre des favoris
	e.FavList.SetBorder(true).SetTitle(" Favoris ").SetBorderColor(tcell.ColorWhite)
	e.FavList.SetSelectedBackgroundColor(tcell.ColorWhite).SetSelectedTextColor(tcell.ColorBlack)
	e.FavList.ShowSecondaryText(false)
	e.FavList.SetFocusFunc(func() {
		e.FavList.SetBorderColor(tcell.ColorYellow)
	})
	e.FavList.SetBlurFunc(func() {
		e.FavList.SetBorderColor(tcell.ColorWhite)
	})

	e.updatePanelFocus()
	e.rebuildMainLayout()
}

// rebuildMainLayout reconstruit l'interface :
// - Si IsDualPane() : Double Panneau (Gauche 50% | Droite 50%)
// - Sinon : Mode par défaut (Explorateur à gauche | Visualiseur à droite)
func (e *EditorApp) rebuildMainLayout() {
	if e.IsDualPane() {
		// MODE DOUBLE PANNEAU
		panelsFlex := tview.NewFlex().SetDirection(tview.FlexColumn)

		if e.ShowFavs {
			panelsFlex.AddItem(e.FavList, 30, 0, false)
		}

		panelsFlex.AddItem(e.LeftPanel.Box, 0, 1, e.ActivePanel == e.LeftPanel)
		panelsFlex.AddItem(e.RightPanel.Box, 0, 1, e.ActivePanel == e.RightPanel)

		mainFlex := tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(e.PathBar, 1, 0, false).
			AddItem(panelsFlex, 0, 1, true).
			AddItem(e.Status, 1, 0, false)

		e.Pages.AddPage("main", mainFlex, true, true)
	} else {
		// MODE PAR DÉFAUT (Explorateur + Visualiseur)
		contentFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
		if e.ShowFavs {
			contentFlex.AddItem(e.FavList, 30, 0, false)
		}
		contentFlex.AddItem(e.LeftPanel.Box, 0, 1, true)
		contentFlex.AddItem(e.Viewer, 0, 2, false) // Visualiseur à droite (environ 66% de largeur)

		mainFlex := tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(e.PathBar, 1, 0, false).
			AddItem(contentFlex, 0, 1, true).
			AddItem(e.Status, 1, 0, false)

		e.Pages.AddPage("main", mainFlex, true, true)
	}
}

// updateStatus affiche un message personnalisé dans la barre d'état, en
// remplacement des raccourcis contextuels.
func (e *EditorApp) updateStatus(msg string) {
	e.statusOverride = msg
	e.Status.SetText(fmt.Sprintf("[yellow]%s", msg))
}

// updateStatusTemp affiche un message temporaire dans la barre d'état pendant
// 5 secondes, puis restaure les raccourcis du contexte actif.
func (e *EditorApp) updateStatusTemp(msg string) {
	e.updateStatus(msg)

	go func() {
		time.Sleep(5 * time.Second)
		e.App.QueueUpdateDraw(func() {
			e.statusOverride = ""
			e.statusMode = e.statusModeFromFocus()
			e.Status.SetText(renderBindingsBar(e.statusMode, e.statusWidth))
		})
	}()
}

// connectRemote connecte le serveur distant sur RightPanel et bascule automatiquement en double panneau.
func (e *EditorApp) connectRemote(proto string, host string, port int, user, pass string) error {
	var remoteFS vfs.VFS
	var err error

	switch proto {
	case "FTP":
		remoteFS, err = vfs.NewFtpFS(host, port, user, pass, false)
	case "FTPS":
		remoteFS, err = vfs.NewFtpFS(host, port, user, pass, true)
	case "SFTP":
		remoteFS, err = vfs.NewSftpFS(host, port, user, pass)
	default:
		return fmt.Errorf("protocole inconnu: %s", proto)
	}

	if err != nil {
		return err
	}

	if ftpFS, ok := remoteFS.(*vfs.FtpFS); ok {
		ftpFS.OnStatus = func(msg string) {
			e.App.QueueUpdateDraw(func() {
				e.updateStatusTemp(msg)
			})
		}
	} else if sftpFS, ok := remoteFS.(*vfs.SftpFS); ok {
		sftpFS.OnStatus = func(msg string) {
			e.App.QueueUpdateDraw(func() {
				e.updateStatusTemp(msg)
			})
		}
	}

	// La connexion distante est affectée à RightPanel
	targetPanel := e.RightPanel
	targetPanel.PreviousFS = targetPanel.FileSystem
	targetPanel.PreviousDir = targetPanel.CurrentDir

	targetPanel.FileSystem = remoteFS
	targetPanel.CurrentDir = "/"
	targetPanel.RemoteLabel = fmt.Sprintf("%s (%s)", proto, host)

	// Bascule automatique vers le mode double panneau
	e.ActivePanel = targetPanel
	e.refreshPanel(targetPanel)
	e.rebuildMainLayout()
	e.updatePanelFocus()
	e.App.SetFocus(targetPanel.List)

	return nil
}

// disconnectRemote ferme la connexion distante et rétablit l'affichage par défaut avec visualiseur.
func (e *EditorApp) disconnectRemote(targetPanel *PanelState) {
	if targetPanel == nil {
		targetPanel = e.RightPanel
	}

	if closer, ok := targetPanel.FileSystem.(io.Closer); ok {
		_ = closer.Close()
	}

	targetPanel.FileSystem = targetPanel.PreviousFS
	if targetPanel.FileSystem == nil {
		targetPanel.FileSystem = &vfs.LocalFS{}
	}
	targetPanel.CurrentDir = targetPanel.PreviousDir
	if targetPanel.CurrentDir == "" {
		wd, _ := filepath.Abs(".")
		targetPanel.CurrentDir = wd
	}
	targetPanel.PreviousFS = nil
	targetPanel.PreviousDir = ""
	targetPanel.RemoteLabel = ""

	// Retour automatique au panneau local et au visualiseur
	e.ActivePanel = e.LeftPanel
	e.rebuildMainLayout()
	e.updatePanelFocus()
	e.App.SetFocus(e.LeftPanel.List)
	e.triggerViewerPreviewForCurrentItem()
	e.updateStatusTemp("[yellow]Déconnecté du serveur distant (visualiseur restauré)")
}
