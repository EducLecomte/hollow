package vfs

import (
	"context"
	"crypto/tls" // Importé pour configurer le chiffrement TLS dans les connexions FTPS
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/jlaffaye/ftp"
)

// cancelableReader enveloppe un io.Reader pour vérifier périodiquement si un contexte est annulé.
type cancelableReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *cancelableReader) Read(p []byte) (n int, err error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(p)
	}
}

// cancelableReadCloser enveloppe un io.ReadCloser pour supporter l'annulation.
type cancelableReadCloser struct {
	ctx    context.Context
	closer io.ReadCloser
}

func (r *cancelableReadCloser) Read(p []byte) (n int, err error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.closer.Read(p)
	}
}

func (r *cancelableReadCloser) Close() error {
	return r.closer.Close()
}

// FtpFS implémente VFS pour le protocole FTP / FTPS avec support de reconnexion.
type FtpFS struct {
	conn     *ftp.ServerConn
	host     string
	port     int
	user     string
	password string
	useTLS   bool         // Indique si la connexion doit utiliser le chiffrement TLS (FTPS)
	OnStatus func(string) // Callback pour notifier l'UI des événements de connexion
}

// NewFtpFS crée une nouvelle connexion FTP/FTPS et retourne une instance de FtpFS.
// Si useTLS est vrai, la connexion sera initialisée avec un chiffrement TLS (FTPS).
func NewFtpFS(host string, port int, user, password string, useTLS bool) (*FtpFS, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	
	// Définition des options par défaut (timeout de connexion)
	opts := []ftp.DialOption{
		ftp.DialWithTimeout(5 * time.Second),
	}
	
	// Si le chiffrement TLS est demandé, on l'ajoute aux options de connexion
	if useTLS {
		opts = append(opts, ftp.DialWithTLS(&tls.Config{
			InsecureSkipVerify: true, // Permet de se connecter même si le certificat n'est pas signé par une autorité publique
		}))
	}

	conn, err := ftp.Dial(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("erreur de connexion: %v", err)
	}

	err = conn.Login(user, password)
	if err != nil {
		_ = conn.Quit()
		return nil, fmt.Errorf("erreur d'authentification: %v", err)
	}

	return &FtpFS{
		conn:     conn,
		host:     host,
		port:     port,
		user:     user,
		password: password,
		useTLS:   useTLS,
	}, nil
}

// ensureConn vérifie la santé de la connexion et tente une reconnexion si nécessaire.
func (f *FtpFS) ensureConn(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	// Test rapide de la connexion avec une commande NoOp
	err := f.conn.NoOp()
	if err == nil {
		return nil
	}

	if f.OnStatus != nil {
		f.OnStatus("[yellow]Reconnexion FTP en cours...")
	}

	// Tentative de reconnexion avec les mêmes paramètres
	addr := fmt.Sprintf("%s:%d", f.host, f.port)
	opts := []ftp.DialOption{
		ftp.DialWithTimeout(5 * time.Second),
	}
	if f.useTLS {
		opts = append(opts, ftp.DialWithTLS(&tls.Config{
			InsecureSkipVerify: true,
		}))
	}

	conn, err := ftp.Dial(addr, opts...)
	if err != nil {
		return fmt.Errorf("reconnexion échouée: %v", err)
	}

	err = conn.Login(f.user, f.password)
	if err != nil {
		_ = conn.Quit()
		return fmt.Errorf("ré-authentification échouée: %v", err)
	}

	f.conn = conn
	if f.OnStatus != nil {
		f.OnStatus("[green]FTP reconnecté")
	}
	return nil
}

func (f *FtpFS) List(ctx context.Context, path string) ([]FileInfo, error) {
	if err := f.ensureConn(ctx); err != nil {
		return nil, err
	}
	entries, err := f.conn.List(path)
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	for _, entry := range entries {
		if entry.Name == "." || entry.Name == ".." {
			continue
		}

		isDir := entry.Type == ftp.EntryTypeFolder
		files = append(files, FileInfo{
			Name:        entry.Name,
			IsDir:       isDir,
			Size:        int64(entry.Size),
			ModTime:     entry.Time,
			Permissions: "rwxr-xr-x",
			Owner:       "ftp",
			Group:       "ftp",
			Mode:        0755,
		})
	}
	return files, nil
}

func (f *FtpFS) Read(ctx context.Context, path string) (io.ReadCloser, error) {
	if err := f.ensureConn(ctx); err != nil {
		return nil, err
	}
	resp, err := f.conn.Retr(path)
	if err != nil {
		return nil, err
	}
	return &cancelableReadCloser{ctx: ctx, closer: resp}, nil
}

func (f *FtpFS) Write(ctx context.Context, path string, data io.Reader) error {
	if err := f.ensureConn(ctx); err != nil {
		return err
	}
	// On utilise un wrapper pour permettre l'annulation durant le transfert
	return f.conn.Stor(path, &cancelableReader{ctx: ctx, reader: data})
}

func (f *FtpFS) Mkdir(ctx context.Context, path string) error {
	if err := f.ensureConn(ctx); err != nil {
		return err
	}
	return f.conn.MakeDir(path)
}

func (f *FtpFS) Copy(ctx context.Context, src, dst string) error {
	return fmt.Errorf("la copie directe n'est pas supportée en FTP, utilisez CopyRecursiveBetweenVFS")
}

func (f *FtpFS) Remove(ctx context.Context, path string) error {
	if err := f.ensureConn(ctx); err != nil {
		return err
	}
	err := f.conn.Delete(path)
	if err != nil {
		err = f.conn.RemoveDirRecur(path)
	}
	return err
}

func (f *FtpFS) Stat(ctx context.Context, path string) (FileInfo, error) {
	if err := f.ensureConn(ctx); err != nil {
		return FileInfo{}, err
	}
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	
	entries, err := f.conn.List(dir)
	if err != nil {
		return FileInfo{}, err
	}

	for _, entry := range entries {
		if entry.Name == name {
			isDir := entry.Type == ftp.EntryTypeFolder
			return FileInfo{
				Name:        entry.Name,
				IsDir:       isDir,
				Size:        int64(entry.Size),
				ModTime:     entry.Time,
				Permissions: "rwxr-xr-x",
				Owner:       "ftp",
				Group:       "ftp",
				Mode:        0755,
			}, nil
		}
	}

	return FileInfo{}, fmt.Errorf("fichier non trouvé")
}

func (f *FtpFS) Close() error {
	return f.conn.Quit()
}

func (f *FtpFS) Chmod(ctx context.Context, path string, mode os.FileMode) error {
	return fmt.Errorf("modification des permissions non supportée via ce client FTP")
}

func (f *FtpFS) Chown(ctx context.Context, path, owner, group string) error {
	return fmt.Errorf("modification du propriétaire non supportée via ce client FTP")
}
