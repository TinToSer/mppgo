package mpp

import "testing"

// \'hh escapes are bytes in the font's code page (\fcharset) or else the
// document's \ansicpg; field results are visible text.
func TestStripRTFCodePagesAndFields(t *testing.T) {
	for in, want := range map[string]string{
		`{\rtf1\ansi\ansicpg1251\deff0{\fonttbl{\f0\fnil\fcharset204 Arial;}}\f0 \'cf\'f0\'e8\'e2\'e5\'f2}`:                    "Привет",
		`{\rtf1\ansi\ansicpg1252\deff0{\fonttbl{\f0\fnil\fcharset0 Arial;}{\f1\fnil\fcharset238 Arial;}}\f0 caf\'e9 \f1 \'9a}`: "café š",
		`{\rtf1\ansi\ansicpg1253 \'e1\'e2}`:                                         "αβ",
		`{\rtf1\ansi{\field{\*\fldinst HYPERLINK "http://x"}{\fldrslt Link text}}}`: "Link text",
		`{\rtf1\ansi{\fonttbl{\f0\fcharset128 MS Gothic;}}\f0 \'82\'a0}`:            "��",
	} {
		if got := stripRTF(in); got != want {
			t.Errorf("stripRTF(%q) = %q, want %q", in, got, want)
		}
	}
}
