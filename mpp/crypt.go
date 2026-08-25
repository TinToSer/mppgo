// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"fmt"

	"github.com/tintoser/mppgo/cfb"
)

// streamSource reads streams from an MPP compound file, transparently
// removing the XOR obfuscation MS Project applies to certain streams when
// the document carries a password flag.
//
// Note this obfuscation is not encryption and knowing the mask requires no
// password: the mask is derived from a plaintext byte in the Props14 stream.
// Genuinely password-protected files (where a read password is set and the
// encryption XML is present) are rejected up front by Read.
type streamSource struct {
	cf        *cfb.File
	obfuscate bool
	mask      byte
}

// encryptionMask derives the XOR mask from the plaintext encryption code
// byte held in Props14. A zero code means no masking is applied.
func encryptionMask(code byte) byte {
	if code == 0 {
		return 0
	}
	return 0xFF - code
}

func newStreamSource(cf *cfb.File, docProps *Props) *streamSource {
	return &streamSource{
		cf:        cf,
		obfuscate: docProps.Byte(propsPasswordFlag) != 0,
		mask:      encryptionMask(byte(docProps.Byte(propsEncryptionCode))),
	}
}

// plain reads a stream verbatim. Used for the streams MS Project leaves
// unobfuscated even in a flagged document: VarMeta, Var2Data, FixedMeta and
// Fixed2Meta.
func (s *streamSource) plain(path string) ([]byte, error) {
	data, err := s.cf.OpenStream(path)
	if err != nil {
		return nil, fmt.Errorf("mpp: reading %s: %w", path, err)
	}
	return data, nil
}

// decoded reads a stream, un-masking it if the document is obfuscated. Used
// for FixedData, Fixed2Data and the per-directory Props streams.
func (s *streamSource) decoded(path string) ([]byte, error) {
	data, err := s.plain(path)
	if err != nil {
		return nil, err
	}
	if !s.obfuscate || s.mask == 0 {
		return data, nil
	}
	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = b ^ s.mask
	}
	return out, nil
}

// has reports whether a stream exists at the given path.
func (s *streamSource) has(path string) bool {
	e, err := s.cf.Lookup(path)
	return err == nil && e.IsStream()
}
