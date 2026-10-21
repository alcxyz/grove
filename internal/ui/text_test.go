package ui

import "testing"

func TestPlainText(t *testing.T) {
	for _, tt := range []struct{ name, input, want string }{
		{"unicode", "Blåbær 世界 👩‍💻", "Blåbær 世界 👩‍💻"},
		{"sgr", "a\x1b[31mred\x1b[0mz", "aredz"},
		{"osc bell", "a\x1b]0;hidden\ab", "ab"},
		{"osc st", "a\x1b]8;;https://example.test\x1b\\link\x1b]8;;\x1b\\b", "alinkb"},
		{"dcs", "a\x1bPpayload\x1b\\b", "ab"},
		{"cursor", "a\x1b[2Jb\x1b[Hc", "abc"},
		{"controls", "a\x00\a\b\r\x7f\u0085b", "ab"},
		{"multiline", "a\n\tb", "a\n\tb"},
		{"incomplete", "a\x1b]0;unfinished", "a"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := PlainText(tt.input); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
	if got := OneLine("å\n\t\x1b[31m界\x1b[0m"); got != "å 界" {
		t.Fatalf("OneLine = %q", got)
	}
}
