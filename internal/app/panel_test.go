package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/EducLecomte/hollow/internal/vfs"
)

func TestPanelStateBasics(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hollow_panel_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	panel := NewPanelState("left", tempDir, &vfs.LocalFS{})
	if panel.ID != "left" {
		t.Errorf("Expected panel ID 'left', got '%s'", panel.ID)
	}
	if panel.DisplayName() != "Local" {
		t.Errorf("Expected display name 'Local', got '%s'", panel.DisplayName())
	}
	if panel.IsRemote() {
		t.Errorf("Expected IsRemote to be false for LocalFS")
	}
	if panel.IsArchive() {
		t.Errorf("Expected IsArchive to be false for LocalFS")
	}

	// Test avec remote label
	panel.RemoteLabel = "FTP (example.com)"
	if panel.DisplayName() != "FTP (example.com)" {
		t.Errorf("Expected remote label display name, got '%s'", panel.DisplayName())
	}

	// Test GetSelectedItem sur liste vide
	if panel.GetSelectedItem() != nil {
		t.Errorf("Expected nil selected item on empty list")
	}

	// Test avec fichiers peuplés
	panel.CurrentFiles = []vfs.FileInfo{
		{Name: "file1.txt", Size: 1024, ModTime: time.Now()},
		{Name: "folder1", IsDir: true, ModTime: time.Now()},
	}
	panel.List.AddItem("..", "", 0, nil)
	panel.List.AddItem("file1.txt", "", 0, nil)
	panel.List.AddItem("folder1/", "", 0, nil)

	panel.List.SetCurrentItem(1) // Index 1 est file1.txt
	item := panel.GetSelectedItem()
	if item == nil || item.Name != "file1.txt" {
		t.Errorf("Expected item 'file1.txt', got %+v", item)
	}

	expectedPath := filepath.Join(tempDir, "file1.txt")
	if panel.GetSelectedPath() != expectedPath {
		t.Errorf("Expected selected path '%s', got '%s'", expectedPath, panel.GetSelectedPath())
	}

	panel.List.SetCurrentItem(0) // Index 0 est ".."
	if panel.GetSelectedItem() != nil {
		t.Errorf("Expected nil item for '..'")
	}
}

func TestEditorAppDualPaneLifecycle(t *testing.T) {
	app := NewEditorApp("")

	// 1. Au démarrage, l'application est en mode simple par défaut (avec visualiseur)
	if app.IsDualPane() {
		t.Errorf("Expected IsDualPane to be false by default")
	}
	if app.ActivePanel != app.LeftPanel {
		t.Errorf("Expected LeftPanel to be active by default")
	}

	// 2. Activation manuelle du mode double panneau
	app.toggleDualPaneMode()
	if !app.IsDualPane() {
		t.Errorf("Expected IsDualPane to be true after toggleDualPaneMode")
	}

	// 3. Bascule de panneau en mode double panneau
	app.SwitchActivePanel()
	if app.ActivePanel != app.RightPanel {
		t.Errorf("Expected RightPanel to be active after SwitchActivePanel")
	}
	if app.InactivePanel() != app.LeftPanel {
		t.Errorf("Expected LeftPanel to be inactive after SwitchActivePanel")
	}

	// 4. Désactivation du mode double panneau -> retour au mode par défaut
	app.toggleDualPaneMode()
	if app.IsDualPane() {
		t.Errorf("Expected IsDualPane to be false after toggleDualPaneMode")
	}
	if app.ActivePanel != app.LeftPanel {
		t.Errorf("Expected LeftPanel to be active after leaving transfer mode")
	}
}
