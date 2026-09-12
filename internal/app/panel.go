package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/EducLecomte/go_hollow_project/internal/utils"
	"github.com/EducLecomte/go_hollow_project/internal/vfs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// PanelState représente l'état complet et les widgets associés à un panneau de navigation (gauche ou droit).
type PanelState struct {
	ID                  string // "left" ou "right"
	FileSystem          vfs.VFS
	CurrentDir          string
	CurrentFiles        []vfs.FileInfo
	List                *tview.List
	InfoBox             *tview.TextView
	Box                 *tview.Flex // Conteneur vertical (List + InfoBox)
	PreviousFS          vfs.VFS
	PreviousDir         string
	initialFileSelected string
	RemoteLabel         string // Ex: "FTP: host", "SFTP: host"
}

// NewPanelState initialise un nouveau panneau de fichiers.
func NewPanelState(id string, initialDir string, fs vfs.VFS) *PanelState {
	if fs == nil {
		fs = &vfs.LocalFS{}
	}
	if initialDir == "" {
		wd, err := filepath.Abs(".")
		if err != nil {
			wd = "/"
		}
		initialDir = wd
	}

	list := tview.NewList()
	list.ShowSecondaryText(false)
	list.SetSelectedBackgroundColor(tcell.ColorWhite).
		SetSelectedTextColor(tcell.ColorBlack)

	infoBox := tview.NewTextView()
	infoBox.SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)

	// Conteneur du panneau avec bordure
	box := tview.NewFlex().SetDirection(tview.FlexRow)
	box.SetBorder(true)

	// Layout interne : Liste en haut (extensible), InfoBox en bas (hauteur fixe 2 lignes)
	box.AddItem(list, 0, 1, true)
	box.AddItem(infoBox, 2, 0, false)

	p := &PanelState{
		ID:           id,
		FileSystem:   fs,
		CurrentDir:   initialDir,
		List:         list,
		InfoBox:      infoBox,
		Box:          box,
		CurrentFiles: nil,
	}

	p.UpdateTitle(false)
	return p
}

// DisplayName renvoie un libellé lisible selon le type de VFS actif sur le panneau.
func (p *PanelState) DisplayName() string {
	if p.RemoteLabel != "" {
		return p.RemoteLabel
	}
	switch p.FileSystem.(type) {
	case *vfs.LocalFS:
		return "Local"
	case *vfs.ArchiveFS:
		return "Archive"
	case *vfs.FtpFS:
		return "FTP"
	case *vfs.SftpFS:
		return "SFTP"
	default:
		return "VFS"
	}
}

// IsRemote indique si le panneau est connecté à un système distant (FTP ou SFTP).
func (p *PanelState) IsRemote() bool {
	switch p.FileSystem.(type) {
	case *vfs.FtpFS, *vfs.SftpFS:
		return true
	default:
		return false
	}
}

// IsArchive indique si le panneau explore une archive virtuelle.
func (p *PanelState) IsArchive() bool {
	_, ok := p.FileSystem.(*vfs.ArchiveFS)
	return ok
}

// GetSelectedItem retourne les métadonnées de l'élément actuellement sélectionné dans la liste.
func (p *PanelState) GetSelectedItem() *vfs.FileInfo {
	idx := p.List.GetCurrentItem()
	if idx <= 0 || p.CurrentFiles == nil || idx-1 >= len(p.CurrentFiles) {
		return nil
	}
	return &p.CurrentFiles[idx-1]
}

// GetSelectedPath retourne le chemin complet de l'élément sélectionné dans le VFS du panneau.
func (p *PanelState) GetSelectedPath() string {
	item := p.GetSelectedItem()
	if item == nil {
		return ""
	}
	return filepath.Join(p.CurrentDir, item.Name)
}

// UpdateTitle met à jour le titre du panneau et la couleur de la bordure selon qu'il est actif ou non.
func (p *PanelState) UpdateTitle(isActive bool) {
	shortPath := utils.ShortenPath(p.CurrentDir)
	var title string
	if isActive {
		p.Box.SetBorderColor(tcell.ColorYellow)
		title = fmt.Sprintf(" [#ffff00]▶ [#00ff00]%s [#ffffff]: [#ffff00]%s ", p.DisplayName(), shortPath)
	} else {
		p.Box.SetBorderColor(tcell.ColorDarkGray)
		title = fmt.Sprintf(" [#888888]%s : %s ", p.DisplayName(), shortPath)
	}
	p.Box.SetTitle(title)
}

// UpdateInfoBox actualise le texte du bloc d'informations au bas du panneau.
func (p *PanelState) UpdateInfoBox(item *vfs.FileInfo) {
	if item == nil {
		p.InfoBox.SetText("[gray].. Dossier parent")
		return
	}

	modTimeStr := item.ModTime.Format("2006-01-02 15:04")
	if item.IsDir {
		p.InfoBox.SetText(fmt.Sprintf(" [green]Dossier [white]| [green]Date: [white]%s [white]| [green]Droits: [white]%s [green]Prop: [white]%s:%s",
			modTimeStr, item.Permissions, item.Owner, item.Group))
	} else {
		p.InfoBox.SetText(fmt.Sprintf(" [green]Taille: [white]%s [white]| [green]Date: [white]%s [white]| [green]Droits: [white]%s [green]Prop: [white]%s:%s",
			utils.FormatSize(item.Size), modTimeStr, item.Permissions, item.Owner, item.Group))
	}
}

// RefreshStyle rafraîchit les couleurs de contraste des dossiers et fichiers de la liste.
func (p *PanelState) RefreshStyle(selectedIndex int) {
	for i := 0; i < p.List.GetItemCount(); i++ {
		m, s := p.List.GetItemText(i)
		if !strings.HasSuffix(m, "/") && !strings.HasPrefix(m, "[#ff8c00]") {
			continue
		}

		name := strings.TrimPrefix(m, "[#ff8c00]")
		if strings.HasSuffix(name, "/") {
			if i == selectedIndex {
				if m != name {
					p.List.SetItemText(i, name, s)
				}
			} else {
				if !strings.HasPrefix(m, "[#ff8c00]") {
					p.List.SetItemText(i, "[#ff8c00]"+name, s)
				}
			}
		}
	}
}
