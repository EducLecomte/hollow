package app

import (
	"context"
	"fmt"
	"os" // Utilisé pour os.Stat afin de vérifier si le chemin est un fichier ou un répertoire
	"path/filepath"
	"strings"
	"time"

	"github.com/EducLecomte/go_hollow_project/internal/utils"
	"github.com/EducLecomte/go_hollow_project/internal/vfs"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type EditorApp struct {
	// Infrastructure Tview
	App   *tview.Application
	Pages *tview.Pages

	// Composants de l'interface principale
	PathBar     *tview.TextView
	FileList    *tview.List
	FileSizeBox *tview.TextView
	Viewer      *tview.TextView
	Status      *tview.TextView
	FavList     *tview.List // Barre latérale des favoris

	// Système de fichiers et Navigation
	FileSystem   vfs.VFS
	CurrentDir   string
	CurrentFiles []vfs.FileInfo
	PreviousFS   vfs.VFS
	PreviousDir  string

	// État de l'éditeur et Presse-papiers
	FilePath      string
	CopiedPath    string
	Clipboard     string
	LastSearch    string
	LastSearchPos int

	// Dossiers favoris
	Favorites []Favorite
	ShowFavs  bool

	// Gestion de l'asynchronisme
	previewCancel context.CancelFunc

	// Chemin du fichier qui doit être sélectionné initialement dans l'explorateur
	initialFileSelected string
}

// NewEditorApp initialise une nouvelle instance de l'application Hollow.
// Elle accepte un paramètre initialPath qui permet de démarrer l'application directement
// sur un dossier spécifique ou d'ouvrir un fichier dès le lancement.
func NewEditorApp(initialPath string) *EditorApp {
	// Définition du système de fichiers local par défaut
	localFS := &vfs.LocalFS{}
	
	// Résolution du répertoire de travail courant (répertoire de base par défaut)
	wd, err := filepath.Abs(".")
	if err != nil {
		wd = "/"
	}

	var fileToOpen string
	var fileSelected string

	// Si un chemin initial a été fourni en paramètre de lancement
	if initialPath != "" {
		// Résolution de son chemin absolu
		absPath, err := filepath.Abs(initialPath)
		if err == nil {
			info, err := os.Stat(absPath)
			if err == nil {
				if info.IsDir() {
					// Si c'est un dossier, on met à jour le répertoire de travail courant
					wd = absPath
				} else {
					// Si c'est un fichier existant, on se place dans son répertoire parent
					// et on enregistre son chemin pour l'ouvrir immédiatement
					wd = filepath.Dir(absPath)
					fileToOpen = absPath
					fileSelected = filepath.Base(absPath)
				}
			} else {
				// Si le fichier/dossier n'existe pas, on suppose que l'utilisateur souhaite créer/éditer un nouveau fichier.
				// On se place donc dans le dossier parent présumé et on enregistre le chemin du fichier.
				wd = filepath.Dir(absPath)
				fileToOpen = absPath
				fileSelected = filepath.Base(absPath)
			}
		}
	}

	e := &EditorApp{
		App:                 tview.NewApplication(),
		PathBar:             tview.NewTextView(),
		FileList:            tview.NewList(),
		FileSizeBox:         tview.NewTextView(),
		Viewer:              tview.NewTextView(),
		Status:              tview.NewTextView(),
		FavList:             tview.NewList(),
		Pages:               tview.NewPages(),
		CurrentDir:          wd,
		FileSystem:          localFS,
		initialFileSelected: fileSelected, // Mémorise le nom du fichier pour le sélectionner dans la liste
	}

	e.loadFavorites()
	e.setupUI()
	e.setupFavHandlers()
	e.setupHandlers()
	e.refreshFileList()

	// Si un fichier doit être ouvert au lancement, on appelle la fonction d'ouverture de fichier
	if fileToOpen != "" {
		e.openFile(fileToOpen, false)
	}

	return e
}

// setupUI configure la disposition des widgets, les styles et les comportements de base de l'interface.
func (e *EditorApp) setupUI() {
	e.PathBar.SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetTextColor(tcell.ColorBlack).
		SetBackgroundColor(tcell.ColorGreen)

	e.FileList.SetBorder(true).SetTitle(" Exploreur ").SetBorderColor(tcell.ColorYellow)
	e.FileList.ShowSecondaryText(false)
	e.FileList.SetSelectedBackgroundColor(tcell.ColorWhite).
		SetSelectedTextColor(tcell.ColorBlack)

	e.FileList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		e.handleFileSelection(index)
	})

	// Gestion dynamique de la couleur des dossiers et mise à jour de l'encart d'info
	isUpdatingList := false
	// Mise à jour asynchrone du visualiseur pour éviter les blocages (surtout en FTP)
	e.FileList.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		// 1. Annulation de la prévisualisation précédente
		if e.previewCancel != nil {
			e.previewCancel()
		}

		if index == 0 {
			e.FileSizeBox.SetText("[gray]Parent Directory")
			e.Viewer.SetText("").SetTitle(" Visualiseur ")
			return
		}

		if e.CurrentFiles == nil || index-1 >= len(e.CurrentFiles) {
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		e.previewCancel = cancel

		file := e.CurrentFiles[index-1]
		modTimeStr := file.ModTime.Format("2006-01-02 15:04")
		path := filepath.Join(e.CurrentDir, file.Name)

		// Mise à jour immédiate des infos basiques (synchrone)
		if file.IsDir {
			e.FileSizeBox.SetText(fmt.Sprintf("[green]Type: [white]Dossier\n[green]Date: [white]%s\n[green]Droits: [white]%s\n[green]Proprio: [white]%s\n[green]Groupe: [white]%s", modTimeStr, file.Permissions, file.Owner, file.Group))
		} else {
			e.FileSizeBox.SetText(fmt.Sprintf("[green]Taille: [white]%s\n[green]Date: [white]%s\n[green]Droits: [white]%s\n[green]Proprio: [white]%s\n[green]Groupe: [white]%s", utils.FormatSize(file.Size), modTimeStr, file.Permissions, file.Owner, file.Group))
		}

		// Prévisualisation asynchrone (E/S et Coloration)
		go func() {
			// Petite pause pour éviter de charger inutilement lors d'un défilement rapide
			time.Sleep(100 * time.Millisecond)
			select {
			case <-ctx.Done():
				return
			default:
			}

			if file.IsDir {
				e.previewDirectory(ctx, path)
			} else {
				e.previewFile(ctx, path)
			}
		}()

		// 2. Gestion dynamique de la couleur des dossiers pour le contraste
		if isUpdatingList {
			return
		}
		isUpdatingList = true
		defer func() { isUpdatingList = false }()

		for i := 0; i < e.FileList.GetItemCount(); i++ {
			m, s := e.FileList.GetItemText(i)
			if !strings.HasSuffix(m, "/") && !strings.HasPrefix(m, "[#ff8c00]") {
				continue
			}

			// Nettoyage du nom
			name := strings.TrimPrefix(m, "[#ff8c00]")
			if strings.HasSuffix(name, "/") {
				if i == index {
					// Sélectionné : pas de tag pour être noir sur blanc
					if m != name {
						e.FileList.SetItemText(i, name, s)
					}
				} else {
					// Non sélectionné : orange
					if !strings.HasPrefix(m, "[#ff8c00]") {
						e.FileList.SetItemText(i, "[#ff8c00]"+name, s)
					}
				}
			}
		}
	})

	e.FileList.SetBorder(true).SetTitle(" Explorateur ").SetBorderColor(tcell.ColorWhite)
	e.FileList.SetSelectedBackgroundColor(tcell.ColorWhite).SetSelectedTextColor(tcell.ColorBlack)
	e.FileList.ShowSecondaryText(false)

	e.FileList.SetFocusFunc(func() {
		e.FileList.SetBorderColor(tcell.ColorYellow)
		e.updateStatus(utils.HelpMsgFiles)
	})
	e.FileList.SetBlurFunc(func() {
		e.FileList.SetBorderColor(tcell.ColorWhite)
	})

	// Encart pour le poids du fichier
	e.FileSizeBox.SetBorder(true).SetTitle(" Info ")
	e.FileSizeBox.SetDynamicColors(true).SetTextAlign(tview.AlignCenter)

	e.Viewer.SetBorder(true).SetTitle(" Visualiseur ").SetBorderColor(tcell.ColorWhite)
	e.Viewer.SetDynamicColors(true).SetRegions(true) // Active le support des couleurs ANSI/Tags
	e.Viewer.SetFocusFunc(func() {
		e.Viewer.SetBorderColor(tcell.ColorYellow)
		e.updateStatus(utils.HelpMsgView)
	})
	e.Viewer.SetBlurFunc(func() {
		e.Viewer.SetBorderColor(tcell.ColorWhite)
	})
	e.Viewer.SetWrap(true)    // Rétablit le retour à la ligne automatique
	e.Viewer.SetDrawFunc(nil) // Supprime la fonction de synchronisation obsolète

	e.Status.SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
	e.updateStatus(utils.HelpMsgDefault)

	// Barre latérale des favoris
	e.FavList.SetBorder(true).SetTitle(" Favoris ").SetBorderColor(tcell.ColorWhite)
	e.FavList.SetSelectedBackgroundColor(tcell.ColorWhite).SetSelectedTextColor(tcell.ColorBlack)
	e.FavList.ShowSecondaryText(false)
	e.FavList.SetFocusFunc(func() {
		e.FavList.SetBorderColor(tcell.ColorYellow)
	})
	e.FavList.SetBlurFunc(func() {
		e.FavList.SetBorderColor(tcell.ColorWhite)
	})

	// Layout principal avec Support de la barre latérale
	e.rebuildMainLayout()
}

// rebuildMainLayout reconstruit l'interface principale avec des proportions équilibrées.
func (e *EditorApp) rebuildMainLayout() {
	// 1. Zone de navigation (Favoris + Explorateur)
	navHorizontalFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
	if e.ShowFavs {
		navHorizontalFlex.AddItem(e.FavList, 0, 2, false) // Proportion 2
	}
	navHorizontalFlex.AddItem(e.FileList, 0, 2, true) // Proportion 2

	// 2. Colonne de navigation complète (Nav + Info en bas)
	navColumn := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(navHorizontalFlex, 0, 1, true).
		AddItem(e.FileSizeBox, 7, 0, false) // Bloc info (7 lignes pour les infos étendues + bordure)

	// 3. Contenu principal (Navigation + Visualiseur)
	contentFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(navColumn, 0, 1, true).
		AddItem(e.Viewer, 0, 2, false) // Le viewer garde une part majoritaire

	// 4. Layout global (PathBar + Contenu + Status)
	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(e.PathBar, 1, 0, false).
		AddItem(contentFlex, 0, 1, true).
		AddItem(e.Status, 1, 0, false)

	e.Pages.AddPage("main", mainFlex, true, true)
}

// updateStatus met à jour le texte de la barre d'état en bas de l'écran.
func (e *EditorApp) updateStatus(msg string) {
	// On s'assure que les messages s'affichent sur une seule ligne
	e.Status.SetText(fmt.Sprintf("[yellow]%s", msg))
}

// updateStatusTemp affiche un message temporaire dans la barre d'état et le restaure après un délai de 5 secondes.
func (e *EditorApp) updateStatusTemp(msg string) {
	e.updateStatus(msg)

	go func() {
		time.Sleep(5 * time.Second)
		// tview n'est pas thread-safe, on utilise QueueUpdateDraw pour mettre à jour l'UI
		e.App.QueueUpdateDraw(func() {
			// Restauration du message d'aide selon le focus actuel
			focus := e.App.GetFocus()
			if focus == e.Viewer {
				e.updateStatus(utils.HelpMsgView)
			} else if _, ok := focus.(*tview.TextArea); ok {
				e.updateStatus(utils.HelpMsgEdit)
			} else {
				// Détection du contexte pour la barre d'état
				if _, ok := e.FileSystem.(*vfs.ArchiveFS); ok {
					e.updateStatus(utils.HelpMsgArchive)
				} else {
					e.updateStatus(utils.HelpMsgFiles)
				}
			}
		})
	}()
}

// connectFTP initialise une connexion à un serveur distant et bascule le système de fichiers de l'application.
func (e *EditorApp) connectFTP(host string, port int, user, pass string) error {
	ftpFS, err := vfs.NewFtpFS(host, port, user, pass)
	if err != nil {
		return err
	}

	// Configuration du callback de statut pour les reconnexions
	ftpFS.OnStatus = func(msg string) {
		e.App.QueueUpdateDraw(func() {
			e.updateStatusTemp(msg)
		})
	}

	// Sauvegarde du système actuel pour permettre le retour
	e.PreviousFS = e.FileSystem
	e.PreviousDir = e.CurrentDir

	e.FileSystem = ftpFS
	e.CurrentDir = "/"
	e.refreshFileList()
	return nil
}
