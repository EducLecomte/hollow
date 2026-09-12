package vfs_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EducLecomte/hollow/internal/vfs"
)

func TestCopyRecursiveBetweenVFS(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hollow_vfs_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	srcDir := filepath.Join(tempDir, "src")
	dstDir := filepath.Join(tempDir, "dst")
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755); err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		t.Fatalf("Failed to create dst dir: %v", err)
	}

	testFile1 := filepath.Join(srcDir, "test1.txt")
	testFile2 := filepath.Join(srcDir, "subdir", "test2.txt")
	if err := os.WriteFile(testFile1, []byte("hello world"), 0644); err != nil {
		t.Fatalf("Failed to write test1: %v", err)
	}
	if err := os.WriteFile(testFile2, []byte("nested file content"), 0644); err != nil {
		t.Fatalf("Failed to write test2: %v", err)
	}

	srcFS := &vfs.LocalFS{}
	dstFS := &vfs.LocalFS{}

	// Test copying directory recursively
	targetPath := filepath.Join(dstDir, "copied_src")
	ctx := context.Background()
	if err := vfs.CopyRecursiveBetweenVFS(ctx, srcFS, dstFS, srcDir, targetPath); err != nil {
		t.Fatalf("CopyRecursiveBetweenVFS failed: %v", err)
	}

	// Verify copied files exist and contents match
	copied1 := filepath.Join(targetPath, "test1.txt")
	copied2 := filepath.Join(targetPath, "subdir", "test2.txt")

	data1, err := os.ReadFile(copied1)
	if err != nil || string(data1) != "hello world" {
		t.Errorf("Copied file 1 mismatch: data=%s, err=%v", string(data1), err)
	}

	data2, err := os.ReadFile(copied2)
	if err != nil || string(data2) != "nested file content" {
		t.Errorf("Copied file 2 mismatch: data=%s, err=%v", string(data2), err)
	}
}

func TestLocalFSOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hollow_localfs_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fs := &vfs.LocalFS{}
	ctx := context.Background()

	// Write
	testFile := filepath.Join(tempDir, "write_test.txt")
	content := "Antigravity VFS test"
	if err := fs.Write(ctx, testFile, strings.NewReader(content)); err != nil {
		t.Fatalf("LocalFS.Write failed: %v", err)
	}

	// Stat
	info, err := fs.Stat(ctx, testFile)
	if err != nil {
		t.Fatalf("LocalFS.Stat failed: %v", err)
	}
	if info.Name != "write_test.txt" || info.IsDir || info.Size != int64(len(content)) {
		t.Errorf("LocalFS.Stat mismatch: %+v", info)
	}

	// Read
	reader, err := fs.Read(ctx, testFile)
	if err != nil {
		t.Fatalf("LocalFS.Read failed: %v", err)
	}
	defer reader.Close()
	readBytes, err := io.ReadAll(reader)
	if err != nil || string(readBytes) != content {
		t.Errorf("LocalFS.Read mismatch: got=%s, want=%s, err=%v", string(readBytes), content, err)
	}

	// Mkdir
	newSubDir := filepath.Join(tempDir, "mysubdir")
	if err := fs.Mkdir(ctx, newSubDir); err != nil {
		t.Fatalf("LocalFS.Mkdir failed: %v", err)
	}

	// List
	entries, err := fs.List(ctx, tempDir)
	if err != nil {
		t.Fatalf("LocalFS.List failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}

	// Remove
	if err := fs.Remove(ctx, testFile); err != nil {
		t.Fatalf("LocalFS.Remove failed: %v", err)
	}
	if _, err := fs.Stat(ctx, testFile); err == nil {
		t.Errorf("File should not exist after Remove")
	}
}
