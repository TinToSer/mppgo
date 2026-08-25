// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp_test

import (
	"bytes"
	"math/rand"
	"os"
	"testing"

	"github.com/tintoser/mppgo/mpp"
)

// FuzzRead checks that Read never panics, whatever bytes it is given.
// Parsing untrusted binary must fail with an error, not a crash.
//
// Run a deeper campaign with:
//
//	go test ./mpp/ -fuzz FuzzRead -fuzztime 5m
func FuzzRead(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
	f.Add(make([]byte, 512))

	// Seed with mutations of the real file when it is available: structurally
	// plausible input reaches far more of the parser than random bytes.
	if original, err := os.ReadFile(sampleFile); err == nil {
		f.Add(original[:4096])
		rng := rand.New(rand.NewSource(7))
		for i := 0; i < 4; i++ {
			seed := append([]byte(nil), original[:16384]...)
			for j := 0; j < 8; j++ {
				seed[rng.Intn(len(seed))] = byte(rng.Intn(256))
			}
			f.Add(seed)
		}
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// A panic fails the test; an error return is the expected outcome.
		_, _ = mpp.Read(bytes.NewReader(data))
	})
}
