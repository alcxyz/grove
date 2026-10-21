package ui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// PlainText removes terminal sequences and controls before trusted styling.
// Newlines and tabs remain available to multiline views.
func PlainText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, ansi.Strip(s))
}

// LineText keeps ordinary spaces in editable input while replacing line breaks and tabs.
func LineText(s string) string {
	return strings.NewReplacer("\n", " ", "\t", " ").Replace(PlainText(s))
}

// OneLine returns plain text with whitespace collapsed for rows and labels.
func OneLine(s string) string {
	return strings.Join(strings.Fields(PlainText(s)), " ")
}
