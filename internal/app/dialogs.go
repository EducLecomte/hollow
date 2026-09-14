package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/EducLecomte/hollow/internal/utils"
	"github.com/EducLecomte/hollow/internal/vfs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// showCenteredDialog est une fonction utilitaire pour positionner un composant au centre de l'écran.
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

// showHelp affiche une fenêtre modale contenant la liste des raccourcis adaptée au contexte.
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

	e.showCenteredDialog("help", helpText, 70, 22)

	helpText.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc || event.Key() == tcell.KeyF1 || event.Rune() == 'q' {
			e.Pages.RemovePage("help")
			if previousFocus != nil {
				e.App.SetFocus(previousFocus)
			}
			return nil
		}
		return event
	})
}

// showQuitConfirmation affiche une boîte de dialogue demandant confirmation avant de quitter.
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

// showDeleteConfirmation affiche une confirmation avant de supprimer un fichier ou un dossier.
func (e *EditorApp) showDeleteConfirmation() {
	p := e.ActivePanel
	item := p.GetSelectedItem()
	if item == nil {
		return
	}
	path := filepath.Join(p.CurrentDir, item.Name)

	modal := tview.NewModal().
		SetText(fmt.Sprintf("Voulez-vous vraiment supprimer %s (%s) ?", item.Name, p.DisplayName())).
		AddButtons([]string{"Supprimer", "Annuler"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Supprimer" {
				e.deleteElement(path)
			}
			e.Pages.RemovePage("delete")
			e.App.SetFocus(p.List)
		})
	e.Pages.AddPage("delete", modal, true, true)
}

// showOverwriteConfirmation demande confirmation avant d'écraser un fichier existant lors d'une copie.
func (e *EditorApp) showOverwriteConfirmation(fileName, destPanelName string, onChoice func(overwrite bool)) {
	previousFocus := e.App.GetFocus()
	modal := tview.NewModal().
		SetText(fmt.Sprintf("L'élément '%s' existe déjà dans la destination (%s).\nVoulez-vous l'écraser ?", fileName, destPanelName)).
		AddButtons([]string{"Écraser", "Annuler"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			e.Pages.RemovePage("overwrite_confirm")
			if buttonLabel == "Écraser" {
				onChoice(true)
			} else {
				onChoice(false)
				if previousFocus != nil {
					e.App.SetFocus(previousFocus)
				}
			}
		})
	e.Pages.AddPage("overwrite_confirm", modal, true, true)
}

// showNewElementDialog affiche une boîte de dialogue pour créer un fichier ou un dossier dans le panneau actif.
func (e *EditorApp) showNewElementDialog() {
	p := e.ActivePanel
	form := tview.NewForm()
	form.AddDropDown("Type", []string{"Fichier", "Dossier"}, 0, nil)
	form.AddInputField("Nom", "", 40, nil, nil)
	form.AddButton("Créer", func() {
		_, elementType := form.GetFormItem(0).(*tview.DropDown).GetCurrentOption()
		name := form.GetFormItem(1).(*tview.InputField).GetText()
		if name == "" {
			return
		}

		e.Pages.RemovePage("new_element")
		if elementType == "Fichier" {
			e.createFile(name)
		} else {
			e.createDir(name)
			e.App.SetFocus(p.List)
		}
	})
	form.AddButton("Annuler", func() {
		e.Pages.RemovePage("new_element")
		e.App.SetFocus(p.List)
	})
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			e.Pages.RemovePage("new_element")
			e.App.SetFocus(p.List)
			return nil
		}
		return event
	})
	form.SetBorder(true).SetTitle(fmt.Sprintf(" Créer un élément [%s] ", p.DisplayName())).SetTitleAlign(tview.AlignCenter)
	e.showCenteredDialog("new_element", form, 60, 9)
}

// showRenameDialog affiche une fenêtre de saisie pour renommer un fichier ou un dossier.
func (e *EditorApp) showRenameDialog() {
	p := e.ActivePanel
	item := p.GetSelectedItem()
	if item == nil {
		return
	}

	oldPath := filepath.Join(p.CurrentDir, item.Name)
	form := tview.NewForm()
	form.AddInputField("Nouveau nom", item.Name, 40, nil, nil)
	form.AddButton("Renommer", func() {
		newName := form.GetFormItem(0).(*tview.InputField).GetText()
		if newName == "" || newName == "." || newName == ".." || strings.ContainsAny(newName, "/\\") {
			e.updateStatusTemp("[red]Nom invalide")
			return
		}
		e.Pages.RemovePage("rename")
		e.renameElement(oldPath, newName)
		e.App.SetFocus(p.List)
	})
	form.AddButton("Annuler", func() {
		e.Pages.RemovePage("rename")
		e.App.SetFocus(p.List)
	})
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			e.Pages.RemovePage("rename")
			e.App.SetFocus(p.List)
			return nil
		}
		return event
	})
	form.SetBorder(true).SetTitle(fmt.Sprintf(" Renommer [%s] ", p.DisplayName())).SetTitleAlign(tview.AlignCenter)
	e.showCenteredDialog("rename", form, 60, 9)
}

// showSymlinkDialog affiche une fenêtre de saisie pour créer un lien symbolique.
func (e *EditorApp) showSymlinkDialog() {
	p := e.ActivePanel
	item := p.GetSelectedItem()
	if item == nil {
		return
	}

	target := filepath.Join(p.CurrentDir, item.Name)
	form := tview.NewForm()
	form.AddInputField("Cible", target, 50, nil, nil)
	form.AddInputField("Nom du lien", item.Name+".link", 40, nil, nil)
	form.AddButton("Créer", func() {
		targetPath := form.GetFormItem(0).(*tview.InputField).GetText()
		linkName := form.GetFormItem(1).(*tview.InputField).GetText()
		if targetPath == "" || linkName == "" || linkName == "." || linkName == ".." || strings.ContainsAny(linkName, "/\\") {
			e.updateStatusTemp("[red]Cible ou nom de lien invalide")
			return
		}
		e.Pages.RemovePage("symlink")
		e.createSymbolicLink(targetPath, linkName)
		e.App.SetFocus(p.List)
	})
	form.AddButton("Annuler", func() {
		e.Pages.RemovePage("symlink")
		e.App.SetFocus(p.List)
	})
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			e.Pages.RemovePage("symlink")
			e.App.SetFocus(p.List)
			return nil
		}
		return event
	})
	form.SetBorder(true).SetTitle(fmt.Sprintf(" Lien symbolique [%s] ", p.DisplayName())).SetTitleAlign(tview.AlignCenter)
	e.showCenteredDialog("symlink", form, 70, 11)
}

// showSaveConfirmation demande confirmation avant de fermer l'éditeur s'il y a des modifications.
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
					e.App.SetFocus(e.ActivePanel.List)
				})
			case "Ignorer":
				e.Pages.RemovePage("edit_screen")
				e.App.SetFocus(e.ActivePanel.List)
			case "Annuler":
			}
			e.Pages.RemovePage("save_confirm")
			if buttonLabel != "Sauvegarder" && buttonLabel != "Ignorer" && previousFocus != nil {
				e.App.SetFocus(previousFocus)
			}
		})

	e.Pages.AddPage("save_confirm", modal, true, true)
}

// showFTPDialog affiche le formulaire de connexion unifié pour accéder à un serveur distant.
func (e *EditorApp) showFTPDialog() {
	form := tview.NewForm()

	// Menu déroulant pour le choix du protocole
	form.AddDropDown("Protocole", []string{"FTP", "FTPS", "SFTP"}, 0, func(option string, optionIndex int) {
		if form.GetFormItemCount() > 2 {
			portField := form.GetFormItem(2).(*tview.InputField)
			if option == "SFTP" {
				portField.SetText("22")
			} else {
				portField.SetText("21")
			}
		}
	})

	form.AddInputField("Hôte", "", 30, nil, nil)
	form.AddInputField("Port", "21", 6, nil, nil)
	form.AddInputField("Utilisateur", "", 30, nil, nil)
	form.AddPasswordField("Mot de passe", "", 30, '*', nil)

	form.AddButton("Se connecter", func() {
		_, proto := form.GetFormItem(0).(*tview.DropDown).GetCurrentOption()
		host := form.GetFormItem(1).(*tview.InputField).GetText()
		portStr := form.GetFormItem(2).(*tview.InputField).GetText()
		user := form.GetFormItem(3).(*tview.InputField).GetText()
		pass := form.GetFormItem(4).(*tview.InputField).GetText()

		if host == "" {
			e.updateStatusTemp("[red]L'hôte est obligatoire")
			return
		}

		var port int
		fmt.Sscanf(portStr, "%d", &port)
		if port == 0 {
			if proto == "SFTP" {
				port = 22
			} else {
				port = 21
			}
		}

		e.Pages.RemovePage("ftp")

		_, cancel := context.WithCancel(context.Background())
		e.showLoadingDialog("Chargement", fmt.Sprintf("Connexion à %s (%s)...", host, proto), cancel)

		go func() {
			err := e.connectRemote(proto, host, port, user, pass)
			e.App.QueueUpdateDraw(func() {
				e.removeLoadingPage()
				if err != nil {
					e.updateStatusTemp(fmt.Sprintf("[red]Erreur %s: %v", proto, err))
				} else {
					e.updateStatusTemp(fmt.Sprintf("[green]Connecté à %s via %s (Double panneau activé)", host, proto))
				}
			})
		}()
	})

	form.AddButton("Annuler", func() {
		e.Pages.RemovePage("ftp")
		e.App.SetFocus(e.ActivePanel.List)
	})

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			e.Pages.RemovePage("ftp")
			e.App.SetFocus(e.ActivePanel.List)
			return nil
		}
		return event
	})

	form.SetBorder(true).SetTitle(" Connexion Réseau (FTP/SFTP) ").SetTitleAlign(tview.AlignCenter)
	e.showCenteredDialog("ftp", form, 52, 17)
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
			e.removeLoadingPage()
		})
	e.loadingModal = modal
	e.Pages.AddPage("loading", modal, true, true)
}

// updateLoadingText met à jour le message de la modale de chargement en cours
// (utilisé par les opérations longues pour afficher l'avancement).
func (e *EditorApp) updateLoadingText(message string) {
	e.App.QueueUpdateDraw(func() {
		if e.loadingModal == nil || !e.Pages.HasPage("loading") {
			e.loadingModal = nil
			return
		}
		e.loadingModal.SetText(message)
	})
}

// removeLoadingPage supprime la modale de chargement et oublie la référence vers elle.
func (e *EditorApp) removeLoadingPage() {
	e.loadingModal = nil
	e.Pages.RemovePage("loading")
}

// showBinaryOpenConfirmation affiche un avertissement avant d'ouvrir un fichier binaire.
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

// showRenameFavoriteDialog affiche une fenêtre de saisie pour renommer un favori.
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

// showChmodDialog affiche une modale pour modifier les permissions Unix d'un élément du panneau actif.
func (e *EditorApp) showChmodDialog() {
	p := e.ActivePanel
	item := p.GetSelectedItem()
	if item == nil {
		return
	}
	path := filepath.Join(p.CurrentDir, item.Name)
	currentMode := fmt.Sprintf("%04o", item.Mode.Perm()&0777)
	currentOwner := item.Owner
	currentGroup := item.Group

	form := tview.NewForm().
		AddInputField("Permissions (octal)", currentMode, 10, nil, nil).
		AddInputField("Propriétaire", currentOwner, 15, nil, nil).
		AddInputField("Groupe", currentGroup, 15, nil, nil)

	if item.IsDir {
		form.AddCheckbox("Récursif (-R)", false, nil)
	}

	form.AddButton("Enregistrer", func() {
		newModeStr := form.GetFormItem(0).(*tview.InputField).GetText()
		newOwner := form.GetFormItem(1).(*tview.InputField).GetText()
		newGroup := form.GetFormItem(2).(*tview.InputField).GetText()

		isRecursive := false
		if item.IsDir {
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
			errChmod = vfs.ChmodRecursive(ctx, p.FileSystem, path, os.FileMode(newMode))
			errChown = vfs.ChownRecursive(ctx, p.FileSystem, path, newOwner, newGroup)
		} else {
			errChmod = p.FileSystem.Chmod(ctx, path, os.FileMode(newMode))
			errChown = p.FileSystem.Chown(ctx, path, newOwner, newGroup)
		}

		if errChmod != nil {
			e.updateStatusTemp(fmt.Sprintf("[red]Erreur chmod: %v", errChmod))
		} else if errChown != nil {
			e.updateStatusTemp(fmt.Sprintf("[red]Erreur chown: %v", errChown))
		} else {
			e.updateStatusTemp("[green]Propriétés modifiées avec succès")
			e.refreshPanel(p)
		}

		e.Pages.RemovePage("chmod")
		e.App.SetFocus(p.List)
	})

	form.AddButton("Annuler", func() {
		e.Pages.RemovePage("chmod")
		e.App.SetFocus(p.List)
	})

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			e.Pages.RemovePage("chmod")
			e.App.SetFocus(p.List)
			return nil
		}
		return event
	})

	form.SetBorder(true).
		SetTitle(fmt.Sprintf(" Propriétés: %s ", item.Name)).
		SetTitleAlign(tview.AlignCenter)

	e.showCenteredDialog("chmod", form, 55, 15)
}
