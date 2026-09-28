// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mspdi

import (
	"bytes"
	"strings"
	"testing"
)

func TestReadWindows1252DeclaredDocument(t *testing.T) {
	// The Author name below is written as raw Windows-1252 bytes containing
	// an e-acute (0xE9, identical in Latin-1 and CP1252) and, to actually
	// exercise the 0x80-0x9F remapping, a right single quotation mark
	// (CP1252 0x92, U+2019) — which is NOT the same byte value in UTF-8.
	var doc bytes.Buffer
	doc.WriteString(`<?xml version="1.0" encoding="windows-1252"?>` + "\n")
	doc.WriteString("<Project><Title>Caf")
	doc.WriteByte(0xE9) // e-acute
	doc.WriteString(" owner")
	doc.WriteByte(0x92) // right single quotation mark
	doc.WriteString("s plan</Title></Project>")

	pf, err := Read(bytes.NewReader(doc.Bytes()))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	want := "Café owner’s plan"
	if pf.Properties.Name != want {
		t.Errorf("Name = %q, want %q", pf.Properties.Name, want)
	}
}

func TestReadUTF8DocumentUnaffectedByCharsetHandling(t *testing.T) {
	// A plain UTF-8 document (the common case, and what every other test in
	// this package uses) must never pass through the Windows-1252 reader —
	// encoding/xml only invokes CharsetReader for a declared non-UTF-8,
	// non-US-ASCII charset, but this pins that assumption down explicitly
	// since a regression here would silently corrupt every non-ASCII
	// character in an ordinary file.
	doc := `<?xml version="1.0" encoding="UTF-8"?><Project><Title>Caf` + "é" + ` owner` + "’" + `s plan</Title></Project>`
	pf, err := Read(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	want := "Café owner’s plan"
	if pf.Properties.Name != want {
		t.Errorf("Name = %q, want %q", pf.Properties.Name, want)
	}
}
