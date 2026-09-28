// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import "testing"

func TestStripRTF(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain text passes through unchanged",
			in:   "just some text",
			want: "just some text",
		},
		{
			name: "simple RTF paragraph",
			in:   `{\rtf1\ansi\ansicpg1252\deff0{\fonttbl{\f0 Calibri;}}\f0 Hello world\par}`,
			want: "Hello world",
		},
		{
			name: "font and colour tables are not emitted",
			in:   `{\rtf1\ansi{\fonttbl{\f0\fnil\fcharset0 Calibri;}}{\colortbl ;\red255\green0\blue0;}\f0 Note text}`,
			want: "Note text",
		},
		{
			name: "par control words become newlines",
			in:   `{\rtf1\ansi Line one\par Line two\par}`,
			want: "Line one\nLine two",
		},
		{
			name: "escaped braces and backslash",
			in:   `{\rtf1\ansi a \{b\} c \\ d}`,
			want: `a {b} c \ d`,
		},
		{
			name: "hex escape decodes as cp1252",
			in:   `{\rtf1\ansi caf\'e9}`,
			want: "café",
		},
		{
			// Built by concatenation, not a literal escape sequence in the
			// source, so nothing along the way "helpfully" decodes it before
			// stripRTF ever sees the text.
			name: "unicode escape with ascii fallback",
			in:   `{\rtf1\ansi ` + string(rune(0x5C)) + `u8364?}`,
			want: string(rune(0x20AC)),
		},
		{
			name: "ignorable destination group is skipped",
			in:   `{\rtf1\ansi{\*\generator Microsoft Project}Visible text}`,
			want: "Visible text",
		},
		{
			name: "tab control word",
			in:   `{\rtf1\ansi a\tab b}`,
			want: "a\tb",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stripRTF(c.in)
			if got != c.want {
				t.Errorf("stripRTF(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// FuzzStripRTF checks that stripRTF never panics on malformed RTF — an
// unmatched brace, a truncated control word or hex escape, or a \u escape
// with no fallback text. Real notes are untrusted binary content in
// disguise, so the parser must degrade, never crash.
func FuzzStripRTF(f *testing.F) {
	f.Add(`{\rtf1\ansi Hello world\par}`)
	f.Add(`{\rtf1\ansi{\fonttbl{\f0 Calibri;}}\f0 text}`)
	f.Add(`{\rtf1\ansi{\*\generator x}荤?}`)
	f.Add(`{\rtf1`)
	f.Add(`{\rtf1\ansi \'`)
	f.Add(`{\rtf1\ansi \u`)
	f.Add(`{`)
	f.Add(`}`)
	f.Add("plain text, not RTF at all")

	f.Fuzz(func(t *testing.T, s string) {
		_ = stripRTF(s)
	})
}
