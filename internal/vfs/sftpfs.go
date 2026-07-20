package vfs

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SftpFS implémente l'interface VFS pour le protocole SFTP (SSH File Transfer Protocol).
type SftpFS struct {
	sshClient  *ssh.Client  // Connexion SSH de base
	sftpClient *sftp.Client // Client SFTP utilisant la session SSH
	host       string
	port       int
	user       string
	password   string
	OnStatus   func(string) // Callback pour notifier l'UI des reconnexions
}

// NewSftpFS crée une nouvelle connexion SSH, initialise le client SFTP et retourne une instance SftpFS.
func NewSftpFS(host string, port int, user, password string) (*SftpFS, error) {
	// Configuration de la connexion SSH (authentification par mot de passe et bypass des clés d'hôte)
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Ignorer la vérification de clé pour la flexibilité TUI
		Timeout:         5 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	sshClient, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("erreur de connexion SSH: %v", err)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return nil, fmt.Errorf("erreur d'initialisation SFTP: %v", err)
	}

	return &SftpFS{
		sshClient:  sshClient,
		sftpClient: sftpClient,
		host:       host,
		port:       port,
		user:       user,
		password:   password,
	}, nil
}

// ensureConn vérifie la santé de la connexion SFTP (via Getwd) et se reconnecte si nécessaire.
func (s *SftpFS) ensureConn(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Un appel Getwd() rapide fait office de NoOp / test de présence
	_, err := s.sftpClient.Getwd()
	if err == nil {
		return nil
	}

	// Notification de reconnexion à l'interface
	if s.OnStatus != nil {
		s.OnStatus("[yellow]Reconnexion SFTP en cours...")
	}

	// Tentative de reconnexion SSH et SFTP
	config := &ssh.ClientConfig{
		User: s.user,
		Auth: []ssh.AuthMethod{
			ssh.Password(s.password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	sshClient, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("reconnexion SSH échouée: %v", err)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return fmt.Errorf("reconnexion SFTP échouée: %v", err)
	}

	s.sshClient = sshClient
	s.sftpClient = sftpClient

	if s.OnStatus != nil {
		s.OnStatus("[green]SFTP reconnecté")
	}
	return nil
}

// List renvoie la liste des fichiers et répertoires présents à un chemin donné.
func (s *SftpFS) List(ctx context.Context, path string) ([]FileInfo, error) {
	if err := s.ensureConn(ctx); err != nil {
		return nil, err
	}

	entries, err := s.sftpClient.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	for _, entry := range entries {
		if entry.Name() == "." || entry.Name() == ".." {
			continue
		}

		// Propriétaire et groupe par défaut
		owner := "sftp"
		group := "sftp"

		// Tenter de récupérer l'UID/GID numérique fourni par SFTP
		if sys := entry.Sys(); sys != nil {
			if fs, ok := sys.(*sftp.FileStat); ok {
				owner = fmt.Sprintf("%d", fs.UID)
				group = fmt.Sprintf("%d", fs.GID)
			}
		}

		files = append(files, FileInfo{
			Name:        entry.Name(),
			IsDir:       entry.IsDir(),
			Size:        entry.Size(),
			ModTime:     entry.ModTime(),
			Permissions: entry.Mode().String(),
			Owner:       owner,
			Group:       group,
			Mode:        entry.Mode(),
		})
	}
	return files, nil
}

// Read ouvre un fichier distant en lecture.
func (s *SftpFS) Read(ctx context.Context, path string) (io.ReadCloser, error) {
	if err := s.ensureConn(ctx); err != nil {
		return nil, err
	}

	file, err := s.sftpClient.Open(path)
	if err != nil {
		return nil, err
	}

	// Wrapper pour prendre en compte l'annulation du contexte pendant la lecture
	return &cancelableReadCloser{ctx: ctx, closer: file}, nil
}

// Write crée ou écrase un fichier distant avec les données fournies.
func (s *SftpFS) Write(ctx context.Context, path string, data io.Reader) error {
	if err := s.ensureConn(ctx); err != nil {
		return err
	}

	file, err := s.sftpClient.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Copie avec support de l'annulation de contexte
	_, err = io.Copy(file, &cancelableReader{ctx: ctx, reader: data})
	return err
}

// Mkdir crée un répertoire distant (y compris ses répertoires parents s'ils manquent).
func (s *SftpFS) Mkdir(ctx context.Context, path string) error {
	if err := s.ensureConn(ctx); err != nil {
		return err
	}
	return s.sftpClient.MkdirAll(path)
}

// Copy indique que le transfert direct SFTP-SFTP sans passer par la mémoire locale n'est pas géré.
func (s *SftpFS) Copy(ctx context.Context, src, dst string) error {
	return fmt.Errorf("la copie directe n'est pas supportée en SFTP, utilisez CopyRecursiveBetweenVFS")
}

// Remove supprime un fichier ou un dossier (de façon récursive).
func (s *SftpFS) Remove(ctx context.Context, path string) error {
	if err := s.ensureConn(ctx); err != nil {
		return err
	}

	info, err := s.sftpClient.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		// Suppression récursive du dossier distant
		return s.removeRecursive(path)
	}
	return s.sftpClient.Remove(path)
}

// removeRecursive supprime tous les sous-fichiers et dossiers enfants avant de supprimer le répertoire racine.
func (s *SftpFS) removeRecursive(path string) error {
	entries, err := s.sftpClient.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		childPath := path + "/" + entry.Name()
		if entry.IsDir() {
			if err := s.removeRecursive(childPath); err != nil {
				return err
			}
		} else {
			if err := s.sftpClient.Remove(childPath); err != nil {
				return err
			}
		}
	}
	return s.sftpClient.RemoveDirectory(path)
}

// Stat récupère les métadonnées d'un fichier/dossier distant.
func (s *SftpFS) Stat(ctx context.Context, path string) (FileInfo, error) {
	if err := s.ensureConn(ctx); err != nil {
		return FileInfo{}, err
	}

	info, err := s.sftpClient.Stat(path)
	if err != nil {
		return FileInfo{}, err
	}

	owner := "sftp"
	group := "sftp"
	if sys := info.Sys(); sys != nil {
		if fs, ok := sys.(*sftp.FileStat); ok {
			owner = fmt.Sprintf("%d", fs.UID)
			group = fmt.Sprintf("%d", fs.GID)
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

// Chmod change les permissions d'un fichier/répertoire distant (supporté nativement par SFTP).
func (s *SftpFS) Chmod(ctx context.Context, path string, mode os.FileMode) error {
	if err := s.ensureConn(ctx); err != nil {
		return err
	}
	return s.sftpClient.Chmod(path, mode)
}

// Chown change le propriétaire et le groupe d'un fichier distant.
// SFTP requiert des UIDs et GIDs numériques, une chaîne non-numérique provoquera une erreur descriptive.
func (s *SftpFS) Chown(ctx context.Context, path, owner, group string) error {
	if err := s.ensureConn(ctx); err != nil {
		return err
	}

	uid := -1
	gid := -1

	// Extraction numérique de l'UID
	if owner != "" {
		if _, err := fmt.Sscanf(owner, "%d", &uid); err != nil {
			return fmt.Errorf("SFTP requiert un UID numérique pour le propriétaire (ex: 1000)")
		}
	}
	// Extraction numérique du GID
	if group != "" {
		if _, err := fmt.Sscanf(group, "%d", &gid); err != nil {
			return fmt.Errorf("SFTP requiert un GID numérique pour le groupe (ex: 1000)")
		}
	}

	return s.sftpClient.Chown(path, uid, gid)
}

// Close libère les ressources et coupe les sessions SFTP et SSH proprement.
func (s *SftpFS) Close() error {
	var errSftp error
	if s.sftpClient != nil {
		errSftp = s.sftpClient.Close()
	}
	var errSsh error
	if s.sshClient != nil {
		errSsh = s.sshClient.Close()
	}
	if errSftp != nil {
		return errSftp
	}
	return errSsh
}
