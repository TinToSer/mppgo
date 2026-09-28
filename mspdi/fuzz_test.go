// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mspdi

import (
	"math/rand"
	"strings"
	"testing"
)

// FuzzRead checks that Read never panics, whatever bytes it is given.
// Parsing untrusted input must fail with an error, not a crash — the same
// standard the mpp package's FuzzRead holds the binary reader to.
//
// Run a deeper campaign with:
//
//	go test ./mspdi/ -fuzz FuzzRead -fuzztime 5m
func FuzzRead(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte(sampleMSPDI))
	f.Add([]byte(`<?xml version="1.0" encoding="windows-1252"?><Project></Project>`))
	f.Add([]byte("not xml at all"))
	f.Add([]byte("<Project>"))

	rng := rand.New(rand.NewSource(11))
	for i := 0; i < 4; i++ {
		seed := []byte(sampleMSPDI)
		mutated := append([]byte(nil), seed...)
		for j := 0; j < 8; j++ {
			mutated[rng.Intn(len(mutated))] = byte(rng.Intn(256))
		}
		f.Add(mutated)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = Read(strings.NewReader(string(data)))
	})
}
