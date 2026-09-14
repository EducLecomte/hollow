package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/EducLecomte/hollow/internal/vfs"
)

// Clés de tri disponibles pour les panneaux de fichiers.
const (
	sortByName = iota // Tri par nom (ordre alphabétique insensible à la casse)
	sortBySize        // Tri par taille
	sortByDate        // Tri par date de modification
)

// sortFiles applique le tri courant au panneau : dossiers d'abord,
// puis fichiers selon le critère choisi (nom, taille ou date).
func (e *EditorApp) sortFiles(files []vfs.FileInfo) {
	sort.SliceStable(files, func(i, j int) bool {
		a, b := files[i], files[j]
		if a.IsDir != b.IsDir {
			// Les dossiers sont toujours regroupés en tête de liste
			return a.IsDir
		}

		var less bool
		switch e.SortKey {
		case sortBySize:
			less = a.Size < b.Size
		case sortByDate:
			less = a.ModTime.Before(b.ModTime)
		default: // sortByName
			less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}

		if e.SortAsc {
			return less
		}
		return !less
	})
}

// sortDescription retourne une description lisible du tri courant pour la barre d'état.
func (e *EditorApp) sortDescription() string {
	var critere string
	switch e.SortKey {
	case sortBySize:
		critere = "taille"
	case sortByDate:
		critere = "date"
	default:
		critere = "nom"
	}
	dir := "croissant"
	if !e.SortAsc {
		dir = "décroissant"
	}
	return fmt.Sprintf("Tri: %s (%s)", critere, dir)
}

// cycleSortKey passe au critère de tri suivant et relance l'affichage des panneaux.
func (e *EditorApp) cycleSortKey() {
	e.SortKey = (e.SortKey + 1) % 3
	e.SortAsc = true
	e.refreshBothPanels()
	e.updateStatusTemp(e.sortDescription())
}

// toggleSortDirection inverse la direction du tri courant.
func (e *EditorApp) toggleSortDirection() {
	e.SortAsc = !e.SortAsc
	e.refreshBothPanels()
	e.updateStatusTemp(e.sortDescription())
}

// toggleHiddenFiles bascule l'affichage des fichiers cachés (nommés d'un point).
func (e *EditorApp) toggleHiddenFiles() {
	e.ShowHidden = !e.ShowHidden
	e.refreshBothPanels()
	if e.ShowHidden {
		e.updateStatusTemp("[green]Fichiers cachés affichés")
	} else {
		e.updateStatusTemp("[yellow]Fichiers cachés masqués")
	}
}
