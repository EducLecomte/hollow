package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/EducLecomte/go_hollow_project/internal/app"
)

// Version est injectée lors de la compilation via -ldflags
var Version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "Affiche la version de Hollow")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Hollow version %s\n", Version)
		os.Exit(0)
	}

	// Récupération de l'argument de chemin initial s'il a été fourni en paramètre
	initialPath := ""
	if flag.NArg() > 0 {
		initialPath = flag.Arg(0)
	}

	e := app.NewEditorApp(initialPath)

	// Sécurité : Restaurer le terminal si l'application crash
	defer func() {
		if r := recover(); r != nil {
			e.App.Stop()
			fmt.Fprintf(os.Stderr, "Erreur fatale : %v\n\nDétails :\n%s", r, debug.Stack())
			os.Exit(1)
		}
	}()

	if err := e.App.SetRoot(e.Pages, true).Run(); err != nil {
		e.App.Stop()
		fmt.Fprintf(os.Stderr, "Erreur d'exécution : %v\n", err)
		os.Exit(1)
	}
}
