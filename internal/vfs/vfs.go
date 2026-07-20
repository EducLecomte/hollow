package vfs

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// FileInfo représente un fichier ou dossier de manière universelle (Local, FTP, Zip)
type FileInfo struct {
	Name        string
	IsDir       bool
	Size        int64
	ModTime     time.Time
	Permissions string
	Owner       string
	Group       string
	Mode        os.FileMode
}

// VFS est l'interface que devront implémenter tes différents modules
type VFS interface {
	List(ctx context.Context, path string) ([]FileInfo, error)
	Read(ctx context.Context, path string) (io.ReadCloser, error)
	Write(ctx context.Context, path string, data io.Reader) error
	Mkdir(ctx context.Context, path string) error
	Copy(ctx context.Context, src, dst string) error
	Remove(ctx context.Context, path string) error
	Stat(ctx context.Context, path string) (FileInfo, error)
	Chmod(ctx context.Context, path string, mode os.FileMode) error
	Chown(ctx context.Context, path, owner, group string) error
	Close() error
}

// LocalFS implémente VFS pour le système de fichiers local
type LocalFS struct{}

// List retourne la liste des fichiers et dossiers présents au chemin donné sur le disque local.
func (l *LocalFS) List(ctx context.Context, path string) ([]FileInfo, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			// Gérer l'erreur ou ignorer le fichier si les infos ne sont pas accessibles
			continue
		}

		// Extraction des infos Unix (Propriétaire et Groupe)
		owner := "unknown"
		group := "unknown"
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			u, err := user.LookupId(fmt.Sprint(stat.Uid))
			if err == nil {
				owner = u.Username
			}
			g, err := user.LookupGroupId(fmt.Sprint(stat.Gid))
			if err == nil {
				group = g.Name
			}
		}

		files = append(files, FileInfo{
			Name:        entry.Name(),
			IsDir:       entry.IsDir(),
			Size:        info.Size(),
			ModTime:     info.ModTime(),
			Permissions: info.Mode().String(),
			Owner:       owner,
			Group:       group,
			Mode:        info.Mode(),
		})
	}
	return files, nil
}

// Read ouvre un fichier local en lecture seule.
func (l *LocalFS) Read(ctx context.Context, path string) (io.ReadCloser, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return os.Open(path)
}

// Write crée ou écrase un fichier local avec les données fournies par le Reader.
func (l *LocalFS) Write(ctx context.Context, path string, data io.Reader) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, data)
	return err
}

// Mkdir crée un nouveau répertoire local avec les permissions par défaut (0755).
func (l *LocalFS) Mkdir(ctx context.Context, path string) error {
	return os.Mkdir(path, 0755)
}

// Copy effectue une copie récursive d'un fichier ou d'un dossier sur le système local.
func (l *LocalFS) Copy(ctx context.Context, src, dst string) error {
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	absDst, err := filepath.Abs(dst)
	if err != nil {
		return err
	}

	// Empêche la copie d'un répertoire dans lui-même
	if strings.HasPrefix(absDst, absSrc+string(filepath.Separator)) || absSrc == absDst {
		return fmt.Errorf("impossible de copier un répertoire dans lui-même ou dans un de ses sous-répertoires")
	}

	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := l.Copy(ctx, filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}

	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// Remove supprime récursivement un fichier ou un répertoire.
func (l *LocalFS) Remove(ctx context.Context, path string) error {
	return os.RemoveAll(path)
}

// Stat retourne les métadonnées (taille, droits, propriétaire) d'un fichier ou dossier local.
func (l *LocalFS) Stat(ctx context.Context, path string) (FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return FileInfo{}, err
	}

	owner := "unknown"
	group := "unknown"
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		u, err := user.LookupId(fmt.Sprint(stat.Uid))
		if err == nil {
			owner = u.Username
		}
		g, err := user.LookupGroupId(fmt.Sprint(stat.Gid))
		if err == nil {
			group = g.Name
		}
	}

	return FileInfo{
		Name:        info.Name(),
		IsDir:       info.IsDir(),
		Size:        info.Size(),
		ModTime:     info.ModTime(),
		Permissions: info.Mode().String(),
		Owner:       owner,
		Group:       group,
		Mode:        info.Mode(),
	}, nil
}

// Close libère les ressources associées au système de fichiers (non requis pour le local).
func (l *LocalFS) Close() error {
	return nil
}

// Chmod modifie les permissions d'un fichier ou répertoire local.
func (l *LocalFS) Chmod(ctx context.Context, path string, mode os.FileMode) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return os.Chmod(path, mode)
}

// Chown modifie le propriétaire et le groupe d'un fichier local.
func (l *LocalFS) Chown(ctx context.Context, path, owner, group string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	uid := -1
	gid := -1
	
	if owner != "" {
		u, err := user.Lookup(owner)
		if err == nil {
			fmt.Sscanf(u.Uid, "%d", &uid)
		}
	}
	if group != "" {
		g, err := user.LookupGroup(group)
		if err == nil {
			fmt.Sscanf(g.Gid, "%d", &gid)
		}
	}
	
	return os.Chown(path, uid, gid)
}

// CopyRecursiveBetweenVFS copie récursivement des fichiers ou répertoires entre deux implémentations différentes de VFS.
func CopyRecursiveBetweenVFS(ctx context.Context, srcFS, dstFS VFS, src, dst string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	info, err := srcFS.Stat(ctx, src)
	if err != nil {
		return err
	}

	if info.IsDir {
		if err := dstFS.Mkdir(ctx, dst); err != nil && !os.IsExist(err) {
			return err
		}
		entries, err := srcFS.List(ctx, src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := CopyRecursiveBetweenVFS(ctx, srcFS, dstFS, filepath.Join(src, entry.Name), filepath.Join(dst, entry.Name)); err != nil {
				return err
			}
		}
		return nil
	}

	reader, err := srcFS.Read(ctx, src)
	if err != nil {
		return err
	}
	defer reader.Close()

	return dstFS.Write(ctx, dst, reader)
}

// ChmodRecursive applique Chmod de manière récursive sur un dossier.
func ChmodRecursive(ctx context.Context, fs VFS, path string, mode os.FileMode) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := fs.Chmod(ctx, path, mode); err != nil {
		return err
	}

	info, err := fs.Stat(ctx, path)
	if err != nil {
		return err
	}

	if info.IsDir {
		entries, err := fs.List(ctx, path)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := ChmodRecursive(ctx, fs, filepath.Join(path, entry.Name), mode); err != nil {
				return err
			}
		}
	}
	return nil
}

// ChownRecursive applique Chown de manière récursive sur un dossier.
func ChownRecursive(ctx context.Context, fs VFS, path, owner, group string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := fs.Chown(ctx, path, owner, group); err != nil {
		return err
	}

	info, err := fs.Stat(ctx, path)
	if err != nil {
		return err
	}

	if info.IsDir {
		entries, err := fs.List(ctx, path)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := ChownRecursive(ctx, fs, filepath.Join(path, entry.Name), owner, group); err != nil {
				return err
			}
		}
	}
	return nil
}
