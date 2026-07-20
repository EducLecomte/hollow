package app

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/EducLecomte/go_hollow_project/internal/utils"
	"github.com/gdamore/tcell/v2"
)

// Favorite représente un marque-page vers un dossier.
type Favorite struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// getFavoritesFilePath retourne le chemin d'accès au fichier de configuration des favoris.
func (e *EditorApp) getFavoritesFilePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		homeDir, _ := os.UserHomeDir()
		configDir = filepath.Join(homeDir, ".config")
	}
	appConfigDir := filepath.Join(configDir, "hollow")
	if _, err := os.Stat(appConfigDir); os.IsNotExist(err) {
		os.MkdirAll(appConfigDir, 0755)
	}
	return filepath.Join(appConfigDir, "favorites.json")
}

// loadFavorites lit le fichier de favoris s'il existe et l'injecte dans la structure de l'application.
func (e *EditorApp) loadFavorites() {
	filePath := e.getFavoritesFilePath()
	data, err := os.ReadFile(filePath)
	if err != nil {
		e.Favorites = []Favorite{}
	} else {
		var favs []Favorite
		if err := json.Unmarshal(data, &favs); err != nil {
			e.Favorites = []Favorite{}
		} else {
			e.Favorites = favs
		}
	}

	// S'assurer que les deux premiers sont Home et Racine
	homeDir, _ := os.UserHomeDir()

	hasHome := false
	hasRoot := false
	if len(e.Favorites) >= 1 && e.Favorites[0].Path == homeDir {
		hasHome = true
	}
	if len(e.Favorites) >= 2 && e.Favorites[1].Path == "/" {
		hasRoot = true
	}

	if !hasHome || !hasRoot {
		// On reconstruit proprement pour que Home soit index 0 et Racine index 1
		newList := []Favorite{
			{Name: "Home", Path: homeDir},
			{Name: "Racine", Path: "/"},
		}
		// On ajoute le reste en filtrant les éventuels doublons de Home/Racine s'ils étaient ailleurs
		for _, f := range e.Favorites {
			if f.Path != homeDir && f.Path != "/" {
				newList = append(newList, f)
			}
		}
		e.Favorites = newList
		e.saveFavorites()
	} else {
		// Même si déjà présents, on s'assure que les noms sont propres (sans emojis)
		e.Favorites[0].Name = "Home"
		e.Favorites[1].Name = "Racine"
		e.saveFavorites()
	}

	e.refreshFavoritesList()
}

// saveFavorites enregistre la liste actuelle de favoris dans le fichier de configuration.
func (e *EditorApp) saveFavorites() {
	filePath := e.getFavoritesFilePath()
	data, err := json.MarshalIndent(e.Favorites, "", "  ")
	if err == nil {
		os.WriteFile(filePath, data, 0644)
	}
}

// addFavorite ajoute ou retire un dossier des favoris (comportement toggle).
func (e *EditorApp) addFavorite(path string) {
	// Vérifier si déjà présent pour le retirer (Toggle)
	for i, f := range e.Favorites {
		if f.Path == path {
			e.Favorites = append(e.Favorites[:i], e.Favorites[i+1:]...)
			e.saveFavorites()
			e.refreshFavoritesList()
			e.updateStatusTemp("[yellow]Retiré des favoris : " + f.Name)
			return
		}
	}

	name := filepath.Base(path)
	if name == "." || name == "/" || name == "" {
		name = "Racine"
	}

	e.Favorites = append(e.Favorites, Favorite{Name: name, Path: path})
	e.saveFavorites()
	e.refreshFavoritesList()
	e.updateStatusTemp("[green]Ajouté aux favoris : " + name)
}

// toggleFavorites affiche ou masque la barre latérale des favoris.
func (e *EditorApp) toggleFavorites() {
	e.ShowFavs = !e.ShowFavs
	e.rebuildMainLayout()
	if e.ShowFavs {
		e.App.SetFocus(e.FavList)
	} else {
		e.App.SetFocus(e.FileList)
	}
}

// refreshFavoritesList met à jour le contenu du widget de liste des favoris.
func (e *EditorApp) refreshFavoritesList() {
	e.FavList.Clear()
	for i, fav := range e.Favorites {
		shortcut := rune(0)
		if i < 9 {
			shortcut = rune('1' + i)
		}
		
		// Au chargement initial, on ne sait pas encore lequel est sélectionné (généralement 0)
		displayName := fav.Name
		if i <= 1 {
			displayName = "[yellow]" + fav.Name
		}
		
		e.FavList.AddItem(displayName, fav.Path, shortcut, nil)
	}
	// On force le style correct pour l'élément sélectionné par défaut
	e.updateFavoritesStyle(e.FavList.GetCurrentItem())
}

// updateFavoritesStyle ajuste dynamiquement les couleurs pour éviter le jaune sur blanc lors de la sélection.
func (e *EditorApp) updateFavoritesStyle(currentIndex int) {
	itemCount := e.FavList.GetItemCount()
	for i, fav := range e.Favorites {
		// Sécurité : ne pas mettre à jour des items qui n'ont pas encore été ajoutés au widget List
		if i >= itemCount {
			break
		}
		
		displayName := fav.Name
		// Si favori système (Home/Racine) et qu'il n'est PAS sélectionné, on le met en jaune
		if i <= 1 && i != currentIndex {
			displayName = "[yellow]" + fav.Name
		}
		// On met à jour l'item dans la liste sans changer le shortcut ni le secondary text
		e.FavList.SetItemText(i, displayName, fav.Path)
	}
}

// setupFavHandlers configure les actions clavier pour la liste des favoris.
func (e *EditorApp) setupFavHandlers() {
	e.FavList.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(e.Favorites) {
			e.FileSizeBox.SetText("[yellow]Favori : [white]" + utils.ShortenPath(e.Favorites[index].Path))
			// Met à jour les couleurs pour éviter le jaune sur fond blanc
			e.updateFavoritesStyle(index)
		}
	})

	e.FavList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index < len(e.Favorites) {
			targetPath := e.Favorites[index].Path

			// Sortie de système de fichiers virtuel si nécessaire
			if e.PreviousFS != nil {
				e.FileSystem = e.PreviousFS
				e.PreviousFS = nil
			}

			e.CurrentDir = targetPath
			e.refreshFileList()
			// e.App.SetFocus(e.FileList) // On garde le focus ici pour permettre une navigation rapide
		}
	})

	e.FavList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			e.App.SetFocus(e.FileList)
			return nil
		case tcell.KeyBacktab:
			e.App.SetFocus(e.Viewer)
			return nil
		case tcell.KeyCtrlX:
			e.showQuitConfirmation()
			return nil
		case tcell.KeyCtrlB, tcell.KeyEsc:
			e.toggleFavorites()
			return nil
		case tcell.KeyDelete, tcell.KeyCtrlR:
			index := e.FavList.GetCurrentItem()
			// Protection des favoris système (0: Home, 1: Racine)
			if index <= 1 {
				e.updateStatusTemp("[red]Les favoris système ne peuvent pas être supprimés")
				return nil
			}
			if index >= 0 && index < len(e.Favorites) {
				e.Favorites = append(e.Favorites[:index], e.Favorites[index+1:]...)
				e.saveFavorites()
				e.refreshFavoritesList()
			}
			return nil
		case tcell.KeyCtrlN:
			index := e.FavList.GetCurrentItem()
				if index <= 1 {
					e.updateStatusTemp("[red]Les favoris système ne peuvent pas être renommés")
					return nil
				}
				if index >= 0 && index < len(e.Favorites) {
					e.showRenameFavoriteDialog(index)
				}
				return nil
		}
		return event
	})
}
