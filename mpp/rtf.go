// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"strconv"
	"strings"
	"unicode/utf16"
)

// MS Project stores task and resource notes as RTF, even when the user
// typed plain text — a bare note comes back wrapped as
// "{\rtf1\ansi...plain text}". stripRTF renders that down to the plain text
// MS Project itself would show in the Notes box, mirroring MPXJ's
// RtfHelper.strip (which delegates to a full third-party RTF parser; this is
// a from-scratch parser covering the constructs Project's own notes writer
// actually emits: font/color tables, paragraph and tab control words, the
// common typographic symbol escapes, and \uNNNN Unicode escapes with their
// ASCII fallback text).
//
// Text that is not formal RTF (does not start "{\rtf") is returned
// unchanged: some fields store plain strings through the same accessor.
func stripRTF(text string) string {
	if !strings.HasPrefix(text, `{\rtf`) {
		return text
	}

	type groupState struct {
		skip      bool // suppress text output while inside this group
		ucSkip    int  // \ucN: ASCII fallback character count following a \u escape
		font      int  // current \fN, which selects the code page of \'hh escapes
		fontTable bool // inside {\fonttbl}, where \fN defines rather than selects
	}

	runes := []rune(text)
	n := len(runes)
	i := 0

	var out []rune
	var stack []groupState
	cur := groupState{ucSkip: 1, font: -1}
	pendingSkip := 0 // remaining \u fallback characters to swallow, not emit
	ansiCodePage := 1252
	fontCodePages := make(map[int]int) // \fN -> code page, from \fcharset
	definingFont := -1

	emit := func(r rune) {
		if pendingSkip > 0 {
			pendingSkip--
			return
		}
		if !cur.skip {
			out = append(out, r)
		}
	}

	readControlWord := func() (word string, hasParam bool, param int) {
		start := i
		for i < n && ((runes[i] >= 'a' && runes[i] <= 'z') || (runes[i] >= 'A' && runes[i] <= 'Z')) {
			i++
		}
		word = string(runes[start:i])
		neg := false
		if i < n && runes[i] == '-' {
			neg = true
			i++
		}
		digitsStart := i
		for i < n && runes[i] >= '0' && runes[i] <= '9' {
			i++
		}
		if i > digitsStart {
			v, _ := strconv.Atoi(string(runes[digitsStart:i]))
			if neg {
				v = -v
			}
			param = v
			hasParam = true
		}
		// A single space after a control word/symbol delimits it without
		// being part of the document text.
		if i < n && runes[i] == ' ' {
			i++
		}
		return word, hasParam, param
	}

	for i < n {
		c := runes[i]
		switch c {
		case '{':
			stack = append(stack, cur)
			i++

		case '}':
			if len(stack) > 0 {
				cur = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
			i++

		case '\r', '\n':
			// Layout whitespace in the RTF source itself, not document text.
			i++

		case '\\':
			i++
			if i >= n {
				break
			}
			switch runes[i] {
			case '\\', '{', '}':
				emit(runes[i])
				i++

			case '\'':
				i++
				if i+2 <= n {
					if v, err := strconv.ParseInt(string(runes[i:i+2]), 16, 32); err == nil {
						codePage := ansiCodePage
						if cp, ok := fontCodePages[cur.font]; ok {
							codePage = cp
						}
						emit(codePageRune(byte(v), codePage))
					}
					i += 2
				}

			case '*':
				// Ignorable destination: the control word or group that
				// follows is meaningful only to applications that recognise
				// it, and produces no visible text otherwise.
				i++
				cur.skip = true

			case '~':
				i++
				emit(' ')

			case '_':
				i++
				emit('-')

			case '-':
				i++ // optional hyphen; not rendered

			default:
				word, hasParam, param := readControlWord()
				switch word {
				case "par", "line":
					emit('\n')
				case "tab":
					emit('\t')
				case "emdash":
					emit('—')
				case "endash":
					emit('–')
				case "lquote":
					emit('‘')
				case "rquote":
					emit('’')
				case "ldblquote":
					emit('“')
				case "rdblquote":
					emit('”')
				case "bullet":
					emit('•')
				case "uc":
					if hasParam {
						cur.ucSkip = param
					}
				case "ansicpg":
					if hasParam {
						ansiCodePage = param
					}
				case "f":
					if hasParam {
						if cur.fontTable {
							definingFont = param
						} else {
							cur.font = param
						}
					}
				case "fcharset":
					if hasParam && cur.fontTable && definingFont >= 0 {
						if cp, ok := charsetCodePages[param]; ok {
							fontCodePages[definingFont] = cp
						}
					}
				case "u":
					if hasParam {
						cp := param
						if cp < 0 {
							cp += 65536
						}
						if !cur.skip {
							codepoint := rune(cp)
							if len(out) > 0 && out[len(out)-1] >= 0xD800 && out[len(out)-1] <= 0xDBFF && codepoint >= 0xDC00 && codepoint <= 0xDFFF {
								out[len(out)-1] = utf16.DecodeRune(out[len(out)-1], codepoint)
							} else {
								out = append(out, codepoint)
							}
						}
						pendingSkip = cur.ucSkip
					}
				case "":
					// Unrecognised control symbol (e.g. a line-end escape
					// already consumed above); nothing to do.
				default:
					if rtfIgnorableDestinations[word] {
						cur.skip = true
					}
					if word == "fonttbl" {
						cur.fontTable = true
					}
				}
			}

		default:
			emit(c)
			i++
		}
	}

	return strings.TrimRight(string(out), "\n")
}

// rtfIgnorableDestinations names control words that introduce a group whose
// content is metadata (font/colour tables, embedded objects, headers and
// footers, ...) rather than the note's own visible text.
var rtfIgnorableDestinations = map[string]bool{
	"fonttbl": true, "colortbl": true, "stylesheet": true, "info": true,
	"generator": true, "pict": true, "object": true, "objdata": true,
	"header": true, "headerf": true, "headerl": true, "headerr": true,
	"footer": true, "footerf": true, "footerl": true, "footerr": true,
	"footnote": true, "annotation": true, "fldinst": true,
	"nonshppict": true, "themedata": true, "colorschememapping": true,
	"datastore": true, "listtable": true, "listoverridetable": true,
	"rsidtbl": true, "xmlnstbl": true, "revtbl": true, "panose": true,
	"falt": true, "latentstyles": true, "template": true,
}

// charsetCodePages maps an RTF \fcharset value to its Windows code page.
// A charset not listed (ANSI, default, symbol) uses the document's
// \ansicpg.
var charsetCodePages = map[int]int{
	128: 932, 129: 949, 134: 936, 136: 950, // double-byte
	161: 1253, 162: 1254, 163: 1258, 177: 1255, 178: 1256, 186: 1257,
	204: 1251, 222: 874, 238: 1250,
}

// codePageRune decodes one \'hh byte in the given Windows code page.
// Double-byte (East Asian) code pages are not decoded: their bytes come in
// pairs this reader has no tables for, so each becomes U+FFFD rather than
// a wrong Latin letter.
func codePageRune(b byte, codePage int) rune {
	if b < 0x80 {
		return rune(b)
	}
	switch codePage {
	case 932, 936, 949, 950:
		return '\uFFFD'
	}
	table, ok := codePages[codePage]
	if !ok {
		table = codePages[1252]
	}
	if r := table[b-0x80]; r != 0 {
		return r
	}
	return rune(b)
}

// cp1252ToRune decodes a byte under the Windows-1252 code page — the
// encoding RTF's \'hh escape almost always carries in a file MS Project
// wrote. Windows-1252 agrees with Latin-1 everywhere except 0x80-0x9F, which
// it uses for characters (smart quotes, dashes, ellipsis) Latin-1 leaves as
// control codes.
func cp1252ToRune(b byte) rune {
	if b < 0x80 || b > 0x9F {
		return rune(b)
	}
	if r, ok := cp1252High[b]; ok {
		return r
	}
	return rune(b)
}

var cp1252High = map[byte]rune{
	0x80: '€', 0x82: '‚', 0x83: 'ƒ', 0x84: '„',
	0x85: '…', 0x86: '†', 0x87: '‡', 0x88: 'ˆ',
	0x89: '‰', 0x8A: 'Š', 0x8B: '‹', 0x8C: 'Œ',
	0x8E: 'Ž', 0x91: '‘', 0x92: '’', 0x93: '“',
	0x94: '”', 0x95: '•', 0x96: '–', 0x97: '—',
	0x98: '˜', 0x99: '™', 0x9A: 'š', 0x9B: '›',
	0x9C: 'œ', 0x9E: 'ž', 0x9F: 'Ÿ',
}
