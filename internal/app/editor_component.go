package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/EducLecomte/hollow/internal/utils"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// showFullEditor initialise et affiche l'interface d'édition avec numérotation des lignes et support des raccourcis de type Nano.
func (e *EditorApp) showFullEditor(content string) {
	initialContent := content
	textArea := tview.NewTextArea()
	textArea.SetText(content, false)
	textArea.SetBorder(true)
	textArea.SetWrap(false) // Désactive le wrap pour aligner les numéros de ligne
	textArea.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorOrange).Foreground(tcell.ColorBlack))

	// Barre latérale pour les numéros de ligne
	lineNumbers := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignRight)
	lineNumbers.SetBackgroundColor(tcell.ColorDefault)

	updateLineNumbers := func() {
		_, _, _, height := textArea.GetInnerRect()
		if height <= 0 {
			height = 40 // Fallback raisonnable
		}
		rowOffset, _ := textArea.GetOffset()
		text := textArea.GetText()
		lines := strings.Split(text, "\n")
		totalLines := len(lines)

		var sb strings.Builder
		sb.WriteString("\n") // Compense la bordure du TextArea
		for i := 0; i < height; i++ {
			lineIdx := rowOffset + i
			if lineIdx < totalLines {
				sb.WriteString(fmt.Sprintf("[#555555]%3d [white]\n", lineIdx+1))
			} else {
				sb.WriteString("\n")
			}
		}
		lineNumbers.SetText(sb.String())
	}

	textArea.SetMovedFunc(func() {
		updateLineNumbers()
	})

	// Appel initial après un court délai pour laisser tview calculer les rects
	go func() {
		time.Sleep(50 * time.Millisecond)
		e.App.QueueUpdateDraw(func() {
			updateLineNumbers()
		})
	}()

	// Fonction pour mettre à jour le titre avec un indicateur de modification
	updateTitle := func(modified bool) {
		status := ""
		if modified {
			status = "[red]*[white] "
		}
		textArea.SetTitle(fmt.Sprintf(" %sÉdition: %s ", status, filepath.Base(e.FilePath)))
	}

	updateTitle(false)

	textArea.SetChangedFunc(func() {
		updateTitle(textArea.GetText() != initialContent)
	})

	// Instructions en bas de l'éditeur (raccourcis adaptés à la largeur)
	footer := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(renderBindingsBar(statusModeEdit, e.statusWidth))
	e.editorFooter = footer

	// Layout principal avec barre latérale
	editorLayout := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(lineNumbers, 4, 0, false).
		AddItem(textArea, 0, 1, true)

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(editorLayout, 0, 1, true).
		AddItem(footer, 1, 0, false)

	e.Pages.AddPage("edit_screen", layout, true, true)
	e.App.SetFocus(textArea)

	lastActionWasCut := false

	textArea.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		key := event.Key()
		if key != tcell.KeyCtrlK {
			lastActionWasCut = false
		}

		if key == tcell.KeyCtrlX || key == tcell.KeyEsc {
			if textArea.GetText() != initialContent {
				e.showSaveConfirmation(textArea.GetText())
			} else {
				e.Pages.RemovePage("edit_screen")
				e.App.SetFocus(e.ActivePanel.List)
			}
			return nil
		}
		if key == tcell.KeyF1 {
			e.showHelp(utils.HelpContentEditor)
			return nil
		}
		if key == tcell.KeyCtrlS {
			content := textArea.GetText()
			e.saveFromFullEditor(content, func() {
				initialContent = content
				updateTitle(false)
			})
			return nil
		}
		if key == tcell.KeyCtrlF {
			e.showSearchDialog(textArea)
			return nil
		}
		if key == tcell.KeyCtrlR {
			e.showReplaceDialog(textArea)
			return nil
		}
		if key == tcell.KeyCtrlK { // Couper la ligne (Nano style)
			text := textArea.GetText()
			lines := strings.Split(text, "\n")
			row, _, _, _ := textArea.GetCursor()
			if row < len(lines) {
				if lastActionWasCut {
					e.Clipboard += "\n" + lines[row]
				} else {
					e.Clipboard = lines[row]
				}
				lastActionWasCut = true

				// Calculate byte offsets for the current line
				start := 0
				for i := 0; i < row; i++ {
					start += len(lines[i]) + 1
				}

				end := start + len(lines[row])
				if row < len(lines)-1 {
					end += 1 // include the trailing newline
				} else if row > 0 {
					start -= 1 // include the preceding newline if last line
				}

				textArea.Replace(start, end, "")
			}
			return nil
		}
		if key == tcell.KeyCtrlU { // Coller la ligne
			if e.Clipboard != "" {
				text := textArea.GetText()
				lines := strings.Split(text, "\n")
				row, _, _, _ := textArea.GetCursor()

				start := 0
				for i := 0; i < row; i++ {
					start += len(lines[i]) + 1
				}

				textArea.Replace(start, start, e.Clipboard+"\n")
			}
			return nil
		}
		return event
	})
}

// saveFromFullEditor écrit le contenu actuel de l'éditeur vers le système de fichiers configuré.
func (e *EditorApp) saveFromFullEditor(content string, onDone func()) {
	ctx, cancel := context.WithCancel(context.Background())
	e.showLoadingDialog("Sauvegarde", fmt.Sprintf("Enregistrement de %s...", filepath.Base(e.FilePath)), cancel)

	go func() {
		reader := strings.NewReader(content)
		err := e.ActivePanel.FileSystem.Write(ctx, e.FilePath, reader)

		e.App.QueueUpdateDraw(func() {
			e.removeLoadingPage()
			if err != nil {
				e.updateStatusTemp(fmt.Sprintf("[red]Erreur de sauvegarde: %v", err))
			} else {
				e.updateStatusTemp(fmt.Sprintf("[green]Enregistré: %s", filepath.Base(e.FilePath)))
			}
		})

		if err == nil {
			e.refreshActivePanel()
			if !e.IsDualPane() {
				e.previewFile(context.Background(), e.ActivePanel.FileSystem, e.FilePath)
			}
			if onDone != nil {
				e.App.QueueUpdateDraw(onDone)
			}
		}
	}()
}

// showSearchDialog affiche une fenêtre de saisie pour rechercher du texte dans le document actuellement ouvert.
func (e *EditorApp) showSearchDialog(textArea *tview.TextArea) {
	inputField := tview.NewInputField().
		SetLabel(" Rechercher: ").
		SetText(e.LastSearch)
	inputField.SetBorder(true).SetTitle(" Recherche ").SetTitleAlign(tview.AlignCenter)

	e.showCenteredDialog("search", inputField, 40, 3)

	inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			term := inputField.GetText()
			if term != "" {
				e.findNext(textArea, term)
			}
		} else if key == tcell.KeyEsc {
			e.Pages.RemovePage("search")
			e.App.SetFocus(textArea)
		}
	})
}

// findNext recherche l'occurrence suivante d'une chaîne, gère la boucle de recherche et centre la vue sur le résultat.
func (e *EditorApp) findNext(textArea *tview.TextArea, term string) {
	text := textArea.GetText()
	if term == "" {
		return
	}

	searchSpace := text
	offset := 0
	if term == e.LastSearch {
		offset = e.LastSearchPos
		if offset < len(text) {
			searchSpace = text[offset:]
		} else {
			offset = 0
		}
	}

	idx := strings.Index(strings.ToLower(searchSpace), strings.ToLower(term))
	if idx == -1 && offset > 0 {
		// On recommence au début (Loop)
		offset = 0
		searchSpace = text
		idx = strings.Index(strings.ToLower(searchSpace), strings.ToLower(term))
	}

	if idx != -1 {
		matchStart := offset + idx
		matchEnd := matchStart + len(term)
		e.LastSearch = term
		e.LastSearchPos = matchEnd

		textArea.Select(matchStart, matchEnd)

		// On calcule la ligne du match pour SetOffset
		linesBefore := strings.Split(text[:matchStart], "\n")
		row := len(linesBefore) - 1

		_, _, _, height := textArea.GetInnerRect()
		if height <= 0 {
			height = 20 // Fallback
		}

		// Centrer le résultat verticalement
		targetOffset := row - (height / 2)
		if targetOffset < 0 {
			targetOffset = 0
		}
		textArea.SetOffset(targetOffset, 0)

		e.updateStatusTemp(fmt.Sprintf("[green]Trouvé: '%s'", term))
	} else {
		e.updateStatusTemp("[yellow]Aucune occurrence trouvée")
	}
}

// showReplaceDialog affiche une fenêtre de remplacement de texte dans le document actuellement ouvert.
// La boîte de dialogue reste ouverte après chaque action pour permettre plusieurs remplacements.
func (e *EditorApp) showReplaceDialog(textArea *tview.TextArea) {
	form := tview.NewForm()
	form.AddInputField("Rechercher", e.LastSearch, 40, nil, nil)
	form.AddInputField("Remplacer par", "", 40, nil, nil)

	closeDialog := func() {
		e.Pages.RemovePage("replace")
		e.App.SetFocus(textArea)
	}

	form.AddButton("Remplacer", func() {
		term := form.GetFormItem(0).(*tview.InputField).GetText()
		if term == "" {
			e.updateStatusTemp("[yellow]Terme de recherche vide")
			return
		}
		replacement := form.GetFormItem(1).(*tview.InputField).GetText()
		e.replaceNext(textArea, term, replacement)
	})
	form.AddButton("Tout remplacer", func() {
		term := form.GetFormItem(0).(*tview.InputField).GetText()
		if term == "" {
			e.updateStatusTemp("[yellow]Terme de recherche vide")
			return
		}
		replacement := form.GetFormItem(1).(*tview.InputField).GetText()
		e.replaceAll(textArea, term, replacement)
	})
	form.AddButton("Annuler", closeDialog)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			closeDialog()
			return nil
		}
		return event
	})
	form.SetBorder(true).SetTitle(" Remplacement ").SetTitleAlign(tview.AlignCenter)
	e.showCenteredDialog("replace", form, 62, 11)
}

// nextMatch retourne les décalages (octets) du début et de la fin de la prochaine
// occurrence insensible à la casse de term dans text, en cherchant à partir de from.
func nextMatch(text, term string, from int) (start, end int, found bool) {
	if term == "" || len(text) == 0 {
		return 0, 0, false
	}
	if from < 0 {
		from = 0
	}
	if from >= len(text) {
		return 0, 0, false
	}
	idx := strings.Index(strings.ToLower(text[from:]), strings.ToLower(term))
	if idx == -1 {
		return 0, 0, false
	}
	start = from + idx
	return start, start + len(term), true
}

// replaceNext remplace la prochaine occurrence du terme recherché et garde la boîte
// de dialogue ouverte. La position du curseur et la vue sont centrées sur le résultat.
func (e *EditorApp) replaceNext(textArea *tview.TextArea, term, replacement string) {
	text := textArea.GetText()

	from := 0
	if term == e.LastSearch && e.LastSearchPos > 0 && e.LastSearchPos <= len(text) {
		from = e.LastSearchPos
	}

	start, end, found := nextMatch(text, term, from)
	if !found {
		start, end, found = nextMatch(text, term, 0)
	}
	if !found {
		e.updateStatusTemp("[yellow]Aucune occurrence trouvée")
		return
	}

	textArea.Replace(start, end, replacement)

	// Sélectionner la zone remplacée et centrer la vue verticalement
	newEnd := start + len(replacement)
	textArea.Select(start, newEnd)

	linesBefore := strings.Split(textArea.GetText()[:start], "\n")
	row := len(linesBefore) - 1
	_, _, _, height := textArea.GetInnerRect()
	if height <= 0 {
		height = 20
	}
	targetOffset := row - (height / 2)
	if targetOffset < 0 {
		targetOffset = 0
	}
	textArea.SetOffset(targetOffset, 0)

	e.LastSearch = term
	e.LastSearchPos = newEnd
	e.updateStatusTemp(fmt.Sprintf("[green]Remplacé: '%s'", term))
}

// replaceAll remplace toutes les occurrences du terme en une seule passe.
// Une seule appel Replace = une seule entrée dans l'historique undo.
func (e *EditorApp) replaceAll(textArea *tview.TextArea, term, replacement string) {
	text := textArea.GetText()
	lowerText := strings.ToLower(text)
	lowerTerm := strings.ToLower(term)

	var sb strings.Builder
	pos := 0
	count := 0
	for {
		idx := strings.Index(lowerText[pos:], lowerTerm)
		if idx == -1 {
			break
		}
		start := pos + idx
		sb.WriteString(text[pos:start])
		sb.WriteString(replacement)
		pos = start + len(term)
		count++
	}
	sb.WriteString(text[pos:])

	if count == 0 {
		e.updateStatusTemp("[yellow]Aucune occurrence trouvée")
		return
	}

	textArea.Replace(0, len(text), sb.String())

	e.LastSearch = term
	e.LastSearchPos = 0
	e.updateStatusTemp(fmt.Sprintf("[green]%d occurrence(s) remplacée(s)", count))
}
