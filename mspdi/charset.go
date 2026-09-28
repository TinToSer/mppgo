// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mspdi

import (
	"bufio"
	"io"
)

// permissiveCharsetReader lets encoding/xml decode a document whose XML
// declaration names an encoding other than UTF-8/US-ASCII (which the
// standard library's Decoder otherwise refuses outright). MSPDI files are
// most commonly declared as UTF-8, but older exports commonly say
// "windows-1252"; every other declared name is treated the same way,
// since — for the plain business text an MSPDI document actually carries —
// the two encodings agree everywhere except the 0x80-0x9F byte range,
// where Windows-1252 has printable characters (smart quotes, an em dash,
// ...) and this is the best guess available without pulling in a real
// charset conversion package (this project has none, by design — see the
// README).
func permissiveCharsetReader(charset string, input io.Reader) (io.Reader, error) {
	return &cp1252Reader{r: bufio.NewReader(input)}, nil
}

type cp1252Reader struct {
	r   *bufio.Reader
	buf [4]byte
	n   int
	pos int
}

func (c *cp1252Reader) Read(p []byte) (int, error) {
	written := 0
	for written < len(p) {
		if c.pos >= c.n {
			b, err := c.r.ReadByte()
			if err != nil {
				if written > 0 {
					return written, nil
				}
				return 0, err
			}
			c.n = encodeRuneUTF8(c.buf[:], cp1252ToRuneMSPDI(b))
			c.pos = 0
		}
		copied := copy(p[written:], c.buf[c.pos:c.n])
		written += copied
		c.pos += copied
	}
	return written, nil
}

// encodeRuneUTF8 writes r's UTF-8 encoding into buf and returns its length.
// buf must be at least 4 bytes (the maximum a single rune ever needs).
func encodeRuneUTF8(buf []byte, r rune) int {
	switch {
	case r < 0x80:
		buf[0] = byte(r)
		return 1
	case r < 0x800:
		buf[0] = 0xC0 | byte(r>>6)
		buf[1] = 0x80 | byte(r&0x3F)
		return 2
	default:
		buf[0] = 0xE0 | byte(r>>12)
		buf[1] = 0x80 | byte((r>>6)&0x3F)
		buf[2] = 0x80 | byte(r&0x3F)
		return 3
	}
}

func cp1252ToRuneMSPDI(b byte) rune {
	if b < 0x80 || b > 0x9F {
		return rune(b)
	}
	if r, ok := cp1252HighMSPDI[b]; ok {
		return r
	}
	return rune(b)
}

var cp1252HighMSPDI = map[byte]rune{
	0x80: '€', 0x82: '‚', 0x83: 'ƒ', 0x84: '„',
	0x85: '…', 0x86: '†', 0x87: '‡', 0x88: 'ˆ',
	0x89: '‰', 0x8A: 'Š', 0x8B: '‹', 0x8C: 'Œ',
	0x8E: 'Ž', 0x91: '‘', 0x92: '’', 0x93: '“',
	0x94: '”', 0x95: '•', 0x96: '–', 0x97: '—',
	0x98: '˜', 0x99: '™', 0x9A: 'š', 0x9B: '›',
	0x9C: 'œ', 0x9E: 'ž', 0x9F: 'Ÿ',
}
