package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/EducLecomte/go_hollow_project/internal/utils"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// showFuzzyFinder affiche une interface de recherche rapide de fichiers dans le dossier courant.
func (e *EditorApp) showFuzzyFinder() {
	var allFiles []string
	e.scanFiles("/", &allFiles)

	if len(allFiles) == 0 {
		e.updateStatusTemp("[yellow]Aucun fichier accessible trouvé sur le système.")
		return
	}

	inputField := tview.NewInputField().
		SetLabel(" Chercher : ").
		SetFieldWidth(0)

	list := tview.NewList().
		ShowSecondaryText(true).
		SetSecondaryTextColor(tcell.ColorGray)

	// Fonction de filtrage
	filter := func(text string) {
		list.Clear()
		count := 0
		tokens := strings.Split(strings.ToLower(text), " ")
		
		for _, path := range allFiles {
			lowerPath := strings.ToLower(path)
			match := true
			for _, token := range tokens {
				if !strings.Contains(lowerPath, token) {
					match = false
					break
				}
			}

			if match {
				list.AddItem(filepath.Base(path), utils.ShortenPath(path), 0, nil)
				count++
				if count > 100 { // Limiter l'affichage pour la performance
					break
				}
			}
		}
	}

	filter("") // Chargement initial

	inputField.SetChangedFunc(filter)

	inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyDown:
			e.App.SetFocus(list)
			return nil
		case tcell.KeyEnter:
			if list.GetItemCount() > 0 {
				_, secondary := list.GetItemText(list.GetCurrentItem())
				// On doit retrouver le chemin réel si on a raccourci avec ~
				fullPath := secondary
				if strings.HasPrefix(secondary, "~/") {
					home, _ := os.UserHomeDir()
					fullPath = filepath.Join(home, secondary[2:])
				} else if secondary == "~" {
					fullPath, _ = os.UserHomeDir()
				}
				
				e.openFile(fullPath, false)
				e.Pages.RemovePage("fuzzy")
			}
			return nil
		case tcell.KeyEscape:
			e.Pages.RemovePage("fuzzy")
			e.App.SetFocus(e.FileList)
			return nil
		}
		return event
	})

	list.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		fullPath := secondaryText
		if strings.HasPrefix(secondaryText, "~/") {
			home, _ := os.UserHomeDir()
			fullPath = filepath.Join(home, secondaryText[2:])
		} else if secondaryText == "~" {
			fullPath, _ = os.UserHomeDir()
		}
		
		e.Pages.RemovePage("fuzzy")
		e.openFile(fullPath, false)
	})

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			e.App.SetFocus(inputField)
			return nil
		}
		return event
	})

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(inputField, 1, 1, true).
		AddItem(tview.NewBox(), 1, 0, false). // Séparateur
		AddItem(list, 0, 10, false)

	flex.SetBorder(true).
		SetTitle(" Recherche Globale (Fuzzy Finder) ").
		SetTitleAlign(tview.AlignCenter)

	e.showCenteredDialog("fuzzy", flex, 80, 20)
}

// scanFiles parcourt récursivement le système de fichiers (VFS) pour collecter les chemins des fichiers.
func (e *EditorApp) scanFiles(dir string, results *[]string) {
	if len(*results) > 10000 {
		return
	}

	files, err := e.FileSystem.List(context.Background(), dir)
	if err != nil {
		return
	}

	for _, f := range files {
		path := filepath.Join(dir, f.Name)
		if f.IsDir {
			// Exclusions système strictes pour la recherche globale
			systemDirs := map[string]bool{
				"proc": true, "sys": true, "dev": true, "run": true, 
				"snap": true, "boot": true, "tmp": true, "node_modules": true,
				"vendor": true, "lost+found": true, ".git": true,
			}
			if systemDirs[f.Name] {
				continue
			}
			e.scanFiles(path, results)
		} else {
			*results = append(*results, path)
		}
	}
}
