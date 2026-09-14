package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	HelpContentExplorer = `
 [yellow]Navigation & Double Panneau[white]
 ---------------------------
 F1		: Afficher cette aide
 F2     : Changer le critère de tri
 F3     : Connexion FTP / SFTP
 F4     : Extraire l'archive vers l'autre panneau
 F5     : Créer un lien symbolique (ln -s)
 F6     : Changer les permissions (Chmod)
 F7     : Créer un fichier ou un dossier
 F8     : Activer le double panneau ou copier vers l'autre panneau
 F9     : Afficher / masquer les fichiers cachés

 [yellow]Panneaux & Navigation[white]
 ---------------------------
 Tab         : Basculer entre panneau gauche et droit
 Ctrl + T    : Activer / désactiver le double panneau local
 Entrée      : Ouvrir un fichier / Entrer dans un dossier
 Suppr       : Supprimer l'élément sélectionné
 
 [yellow]Explorateur & Fichiers[white]
 ----------------------
 Ctrl + B    : Afficher / Masquer la barre latérale des favoris
 Ctrl + D    : Ajouter le dossier courant des favoris
 Ctrl + F    : Recherche Globale (Fuzzy Finder sur tout le disque)
 Ctrl + K/U  : Copier / Coller (mémorisation de chemin)
 Ctrl + R    : Renommer le fichier ou dossier sélectionné
 Ctrl + X    : Quitter l'application

  [yellow]Tri & Affichage[white]
  -------------------------
  F2          : Changer le critère de tri (nom / taille / date)
  Ctrl + O    : Inverser le sens du tri (croissant / décroissant)
	F9 / Ctrl+H : Afficher / masquer les fichiers cachés

  [yellow]Barre des Favoris (Active)[white]
  --------------------------
  1 - 9       : Accès rapide direct au favori par son numéro
  Ctrl + N    : Renommer le favori sélectionné
  Suppr       : Supprimer le favori de la liste
  `

	HelpContentArchive = `
 [yellow]Exploration d'Archive[white]
 ----------------------
 F1     : Afficher cette aide
 F4     : Extraire l'élément sélectionné vers le dossier hôte

 [yellow]Navigation[white]
 ----------------------
 Entrée      : Visualiser un fichier dans l'archive
 TAB/S-TAB   : Naviguer entre les panneaux (Cycle)
 `

	HelpContentEditor = `
  [yellow]Édition de Texte[white]
  ----------------
  F1          : Afficher cette aide
  Ctrl + F    : Rechercher (Entrée pour suivant)
  Ctrl + R    : Remplacer (occurrence suivante ou tout remplacer)
  Ctrl + K    : Couper la ligne (Nano style, concatène si répété)
  Ctrl + S    : Sauvegarder les modifications
  Ctrl + U    : Coller le texte ou le bloc coupé
  Ctrl + Z    : Annuler la dernière action
  Ctrl + Y    : Rétablir l'action annulée
  Ctrl + X    : Fermer l'éditeur (confirmation si modifié)
  
  [yellow]Déplacement[white]
  -----------
  Flèches     : Se déplacer dans le texte
  Page Up/Down: Défilement rapide
  `
)

func FormatSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func IsArchive(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".zip" || ext == ".tar" || ext == ".gz" || ext == ".tgz" {
		return true
	}
	// Check for double extension like .tar.gz
	if strings.HasSuffix(strings.ToLower(name), ".tar.gz") {
		return true
	}
	return false
}

// IsBinary détecte si un contenu est binaire en cherchant des octets nuls (null bytes) dans les premiers octets.
func IsBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	// On analyse un échantillon du début du fichier
	limit := 1024
	if len(data) < limit {
		limit = len(data)
	}

	for i := 0; i < limit; i++ {
		if data[i] == 0 { // Un octet nul est le signe quasi certain d'un fichier binaire
			return true
		}
	}
	return false
}

// GetBinaryFileDescription renvoie une description amicale du type de fichier basé sur l'extension.
func GetBinaryFileDescription(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	// Images
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp", ".tiff", ".svg", ".ico":
		return "une image"
	// Archives
	case ".zip", ".tar", ".gz", ".tgz", ".rar", ".7z", ".xz", ".bz2":
		return "une archive compressée"
	// Executables/Librairies
	case ".exe", ".dll", ".so", ".dylib", ".bin", ".elf", ".out":
		return "un exécutable ou une bibliothèque système"
	// Vidéos
	case ".mp4", ".mkv", ".avi", ".mov", ".flv", ".webm":
		return "une vidéo"
	// Audios
	case ".mp3", ".wav", ".ogg", ".flac", ".aac":
		return "un fichier audio"
	// Documents/Base de données
	case ".pdf":
		return "un document PDF"
	case ".epub", ".mobi":
		return "un livre numérique"
	case ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".odt", ".ods":
		return "un document bureautique"
	case ".sqlite", ".sqlite3", ".db":
		return "une base de données"
	case ".iso", ".img":
		return "une image disque"
	default:
		return "un fichier binaire ou non textuel"
	}
}

// ShortenPath remplace le chemin du répertoire utilisateur par ~ si applicable.
func ShortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	if strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
