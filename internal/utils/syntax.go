package utils

import (
	"bytes"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// Highlight transforme un code source en texte formaté avec des couleurs ANSI pour tview
func Highlight(code, filename string) string {
	// 1. Détecter le langage
	lexer := lexers.Get(filename)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	// 2. Choisir un style (monokai, solarized-dark, dracula, etc.)
	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

	// 3. Utiliser le formateur de terminal (compatible avec tview.TextView)
	formatter := formatters.Get("terminal16m")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	contents := new(bytes.Buffer)
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code
	}

	_ = formatter.Format(contents, style, iterator)
	return contents.String()
}
