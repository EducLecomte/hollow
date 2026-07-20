package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/EducLecomte/go_hollow_project/internal/utils"
	"github.com/EducLecomte/go_hollow_project/internal/vfs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// showCenteredDialog est une fonction utilitaire pour positionner n'importe quel composant au centre de l'écran par-dessus l'interface principale.
func (e *EditorApp) showCenteredDialog(pageName string, item tview.Primitive, width, height int) {
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(item, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)

	e.Pages.AddPage(pageName, flex, true, true)
	e.App.SetFocus(item)
}

// showHelp affiche une fenêtre modale contenant la liste des raccourcis adaptée au contexte passé en paramètre.
func (e *EditorApp) showHelp(content string) {
	previousFocus := e.App.GetFocus()
	helpText := tview.NewTextView().
		SetText(content).
		SetDynamicColors(true).
		SetScrollable(true).
		SetTextAlign(tview.AlignLeft)

	helpText.SetBorder(true).
		SetTitle(" Aide Hollow - Raccourcis ").
		SetTitleAlign(tview.AlignCenter).
		SetBorderPadding(1, 1, 2, 2)

	e.showCenteredDialog("help", helpText, 65, 20)

	helpText.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc || event.Key() == tcell.KeyCtrlG || event.Rune() == 'q' {
			e.Pages.RemovePage("help")
			if previousFocus != nil {
				e.App.SetFocus(previousFocus)
			}
			return nil
		}
		return event
	})
}

// showQuitConfirmation affiche une boîte de dialogue demandant confirmation avant de quitter définitivement l'application.
func (e *EditorApp) showQuitConfirmation() {
	previousFocus := e.App.GetFocus()
	modal := tview.NewModal().
		SetText("Voulez-vous vraiment quitter Hollow ?").
		AddButtons([]string{"Oui", "Non"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Oui" {
				e.saveLastDir()
				e.App.Stop()
			}
			e.Pages.RemovePage("quit")
			if previousFocus != nil {
				e.App.SetFocus(previousFocus)
			}
		})
	e.Pages.AddPage("quit", modal, true, true)
}

// showDeleteConfirmation affiche une confirmation avant de supprimer physiquement un fichier ou un dossier.
func (e *EditorApp) showDeleteConfirmation() {
	index := e.FileList.GetCurrentItem()
	if index <= 0 || index-1 >= len(e.CurrentFiles) {
		return
	}
	file := e.CurrentFiles[index-1]
	path := filepath.Join(e.CurrentDir, file.Name)

	modal := tview.NewModal().
		SetText(fmt.Sprintf("Voulez-vous vraiment supprimer %s ?", file.Name)).
		AddButtons([]string{"Supprimer", "Annuler"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Supprimer" {
				e.deleteElement(path)
			}
			e.Pages.RemovePage("delete")
			e.App.SetFocus(e.FileList)
		})
	e.Pages.AddPage("delete", modal, true, true)
}

// showNewFileDialog affiche une boîte de saisie pour nommer et créer un nouveau fichier dans le dossier courant.
func (e *EditorApp) showNewFileDialog() {
	inputField := tview.NewInputField().SetLabel(" Nom du nouveau fichier: ")
	inputField.SetBorder(true).SetTitle(" Nouveau Fichier ").SetTitleAlign(tview.AlignCenter)

	e.showCenteredDialog("newfile", inputField, 60, 3)

	inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			name := inputField.GetText()
			if name != "" {
				e.createFile(name)
				e.App.SetFocus(e.Viewer)
			}
			e.Pages.RemovePage("newfile")
		} else if key == tcell.KeyEsc {
			e.Pages.RemovePage("newfile")
			e.App.SetFocus(e.FileList)
		}
	})
}

// showSaveConfirmation demande à l'utilisateur s'il souhaite sauvegarder ses modifications avant de fermer l'éditeur plein écran.
func (e *EditorApp) showSaveConfirmation(content string) {
	previousFocus := e.App.GetFocus()
	modal := tview.NewModal().
		SetText("Voulez-vous sauvegarder les modifications avant de quitter ?").
		AddButtons([]string{"Sauvegarder", "Ignorer", "Annuler"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			switch buttonLabel {
			case "Sauvegarder":
				e.saveFromFullEditor(content, func() {
					e.Pages.RemovePage("edit_screen")
					e.App.SetFocus(e.FileList)
				})
			case "Ignorer":
				e.Pages.RemovePage("edit_screen")
				e.App.SetFocus(e.FileList)
			case "Annuler":
				// On ferme juste la modale, le focus revient à l'éditeur
			}
			e.Pages.RemovePage("save_confirm")
			if buttonLabel != "Sauvegarder" && buttonLabel != "Ignorer" && previousFocus != nil {
				e.App.SetFocus(previousFocus)
			}
		})

	e.Pages.AddPage("save_confirm", modal, true, true)
}

// showNewDirDialog affiche une boîte de saisie pour créer un nouveau répertoire.
func (e *EditorApp) showNewDirDialog() {
	inputField := tview.NewInputField().SetLabel(" Nom du nouveau dossier: ")
	inputField.SetBorder(true).SetTitle(" Nouveau Dossier ").SetTitleAlign(tview.AlignCenter)

	e.showCenteredDialog("newdir", inputField, 60, 3)

	inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			name := inputField.GetText()
			if name != "" {
				e.createDir(name)
			}
			e.Pages.RemovePage("newdir")
			e.App.SetFocus(e.FileList)
		} else if key == tcell.KeyEsc {
			e.Pages.RemovePage("newdir")
			e.App.SetFocus(e.FileList)
		}
	})
}

// showFTPDialog affiche le formulaire de connexion pour accéder à un serveur distant via le protocole FTP.
func (e *EditorApp) showFTPDialog() {
	form := tview.NewForm()
	form.AddInputField("Hôte", "", 30, nil, nil)
	form.AddInputField("Port", "21", 6, nil, nil)
	form.AddInputField("Utilisateur", "", 30, nil, nil)
	form.AddPasswordField("Mot de passe", "", 30, '*', nil)

	form.AddButton("Se connecter", func() {
		host := form.GetFormItem(0).(*tview.InputField).GetText()
		portStr := form.GetFormItem(1).(*tview.InputField).GetText()
		user := form.GetFormItem(2).(*tview.InputField).GetText()
		pass := form.GetFormItem(3).(*tview.InputField).GetText()

		if host == "" {
			e.updateStatusTemp("[red]L'hôte est obligatoire")
			return
		}

		var port int
		fmt.Sscanf(portStr, "%d", &port)
		if port == 0 {
			port = 21
		}

		e.Pages.RemovePage("ftp")

		// Affichage de la modale de chargement pendant la connexion
		_, cancel := context.WithCancel(context.Background())
		e.showLoadingDialog("Chargement", fmt.Sprintf("Connexion à %s...", host), cancel)

		go func() {
			err := e.connectFTP(host, port, user, pass)
			e.App.QueueUpdateDraw(func() {
				e.Pages.RemovePage("loading")
				if err != nil {
					e.updateStatusTemp(fmt.Sprintf("[red]Erreur FTP: %v", err))
				} else {
					e.updateStatusTemp(fmt.Sprintf("[green]Connecté avec succès à %s", host))
				}
			})
		}()
	})

	form.AddButton("Annuler", func() {
		e.Pages.RemovePage("ftp")
		e.App.SetFocus(e.FileList)
	})

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			e.Pages.RemovePage("ftp")
			e.App.SetFocus(e.FileList)
			return nil
		}
		return event
	})

	form.SetBorder(true).SetTitle(" Connexion FTP ").SetTitleAlign(tview.AlignCenter)
	e.showCenteredDialog("ftp", form, 50, 15)
}

// showLoadingDialog affiche une modale d'attente pour les opérations longues avec option d'annulation.
func (e *EditorApp) showLoadingDialog(title string, message string, cancelFunc context.CancelFunc) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"Annuler"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if (buttonLabel == "Annuler" || buttonLabel == "") && cancelFunc != nil {
				cancelFunc()
			}
			e.Pages.RemovePage("loading")
		})
	e.Pages.AddPage("loading", modal, true, true)
}

// showBinaryOpenConfirmation affiche un avertissement avant d'ouvrir un fichier détecté comme binaire ou non-textuel.
func (e *EditorApp) showBinaryOpenConfirmation(path string, onConfirm func()) {
	previousFocus := e.App.GetFocus()
	fileName := filepath.Base(path)
	fileDescription := utils.GetBinaryFileDescription(fileName)
	
	modal := tview.NewModal().
		SetText(fmt.Sprintf("Le fichier %s semble être %s. L'ouvrir peut causer des instabilités ou un affichage illisible.\n\nVoulez-vous continuer ?", fileName, fileDescription)).
		AddButtons([]string{"Ouvrir", "Annuler"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Ouvrir" {
				onConfirm()
			}
			e.Pages.RemovePage("binary_confirm")
			if buttonLabel != "Ouvrir" && previousFocus != nil {
				e.App.SetFocus(previousFocus)
			}
		})
	e.Pages.AddPage("binary_confirm", modal, true, true)
}
// showRenameFavoriteDialog affiche une fenêtre de saisie pour donner un nom personnalisé à un favori.
func (e *EditorApp) showRenameFavoriteDialog(index int) {
	fav := e.Favorites[index]
	inputField := tview.NewInputField().
		SetLabel(" Nouveau nom : ").
		SetText(fav.Name)

	inputField.SetBorder(true).
		SetTitle(" Renommer le favori ").
		SetTitleAlign(tview.AlignCenter)

	e.showCenteredDialog("rename_fav", inputField, 60, 3)

	inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			newName := inputField.GetText()
			if newName != "" {
				e.Favorites[index].Name = newName
				e.saveFavorites()
				e.refreshFavoritesList()
			}
			e.Pages.RemovePage("rename_fav")
			e.App.SetFocus(e.FavList)
		} else if key == tcell.KeyEscape {
			e.Pages.RemovePage("rename_fav")
			e.App.SetFocus(e.FavList)
		}
	})
}

// showChmodDialog affiche une fenêtre pour modifier les permissions Unix d'un fichier/dossier.
func (e *EditorApp) showChmodDialog() {
	index := e.FileList.GetCurrentItem()
	if index <= 0 || index-1 >= len(e.CurrentFiles) {
		return
	}
	file := e.CurrentFiles[index-1]
	path := filepath.Join(e.CurrentDir, file.Name)
	currentMode := fmt.Sprintf("%04o", file.Mode.Perm() & 0777)
	currentOwner := file.Owner
	currentGroup := file.Group

	form := tview.NewForm().
		AddInputField("Permissions (octal)", currentMode, 10, nil, nil).
		AddInputField("Propriétaire", currentOwner, 15, nil, nil).
		AddInputField("Groupe", currentGroup, 15, nil, nil)

	if file.IsDir {
		form.AddCheckbox("Récursif (-R)", false, nil)
	}

	form.AddButton("Enregistrer", func() {
		newModeStr := form.GetFormItem(0).(*tview.InputField).GetText()
		newOwner := form.GetFormItem(1).(*tview.InputField).GetText()
		newGroup := form.GetFormItem(2).(*tview.InputField).GetText()

		isRecursive := false
		if file.IsDir {
			isRecursive = form.GetFormItem(3).(*tview.Checkbox).IsChecked()
		}

		newMode, err := strconv.ParseUint(newModeStr, 8, 32)
		if err != nil {
			e.updateStatusTemp("[red]Format octal invalide")
			return
		}

		ctx := context.Background()
		var errChmod, errChown error

		if isRecursive {
			errChmod = vfs.ChmodRecursive(ctx, e.FileSystem, path, os.FileMode(newMode))
			errChown = vfs.ChownRecursive(ctx, e.FileSystem, path, newOwner, newGroup)
		} else {
			errChmod = e.FileSystem.Chmod(ctx, path, os.FileMode(newMode))
			errChown = e.FileSystem.Chown(ctx, path, newOwner, newGroup)
		}

		if errChmod != nil {
			e.updateStatusTemp(fmt.Sprintf("[red]Erreur chmod: %v", errChmod))
		} else if errChown != nil {
			e.updateStatusTemp(fmt.Sprintf("[red]Erreur chown: %v", errChown))
		} else {
			e.updateStatusTemp("[green]Propriétés modifiées avec succès")
			e.refreshFileList()
		}
		
		e.Pages.RemovePage("chmod")
		e.App.SetFocus(e.FileList)
	})

	form.AddButton("Annuler", func() {
		e.Pages.RemovePage("chmod")
		e.App.SetFocus(e.FileList)
	})

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			e.Pages.RemovePage("chmod")
			e.App.SetFocus(e.FileList)
			return nil
		}
		return event
	})

	form.SetBorder(true).
		SetTitle(fmt.Sprintf(" Propriétés: %s ", file.Name)).
		SetTitleAlign(tview.AlignCenter)

	e.showCenteredDialog("chmod", form, 55, 15)
}
