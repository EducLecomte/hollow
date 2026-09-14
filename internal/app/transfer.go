package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/EducLecomte/hollow/internal/utils"
	"github.com/EducLecomte/hollow/internal/vfs"
)

// progressState agrège les statistiques d'un transfert (fichiers et octets transférés).
type progressState struct {
	mu          sync.Mutex
	filesTotal  int
	filesCopied int
	bytesCopied int64
}

func (p *progressState) addFile() {
	p.mu.Lock()
	p.filesTotal++
	p.mu.Unlock()
}

func (p *progressState) fileDone() {
	p.mu.Lock()
	p.filesCopied++
	p.mu.Unlock()
}

func (p *progressState) addBytes(n int) {
	p.mu.Lock()
	p.bytesCopied += int64(n)
	p.mu.Unlock()
}

// summary retourne un résumé d'avancement lisible.
func (p *progressState) summary() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return fmt.Sprintf("%d/%d fichier(s) (%s)", p.filesCopied, p.filesTotal, utils.FormatSize(p.bytesCopied))
}

// progressReader enveloppe un io.Reader afin de comptabiliser les octets lus.
type progressReader struct {
	reader io.Reader
	state  *progressState
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.state.addBytes(n)
	}
	return n, err
}

// copyTreeWithProgress copie récursivement un fichier ou un dossier entre deux VFS
// en rapportant l'avancement (fichiers et octets) dans la progressState fournie.
func copyTreeWithProgress(ctx context.Context, srcFS, dstFS vfs.VFS, src, dst string, state *progressState) error {
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
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if err := copyTreeWithProgress(ctx, srcFS, dstFS, filepath.Join(src, entry.Name), filepath.Join(dst, entry.Name), state); err != nil {
				return err
			}
		}
		return nil
	}

	state.addFile()
	reader, err := srcFS.Read(ctx, src)
	if err != nil {
		return err
	}
	defer reader.Close()

	if err := dstFS.Write(ctx, dst, &progressReader{reader: reader, state: state}); err != nil {
		return err
	}
	state.fileDone()
	return nil
}

// startProgressTicker met à jour la modale de chargement toutes les 200 ms
// avec le résumé d'avancement, jusqu'à l'annulation du contexte.
func (e *EditorApp) startProgressTicker(ctx context.Context, state *progressState, header string) {
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.updateLoadingText(header + "\n\n" + state.summary())
			}
		}
	}()
}
