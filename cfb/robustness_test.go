// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package cfb_test

import (
	"bytes"
	"math/rand"
	"os"
	"testing"

	"github.com/tintoser/mppgo/cfb"
)

// Open must reject or safely handle arbitrary input rather than panicking,
// looping forever, or attempting an enormous allocation.
func TestOpenRejectsGarbage(t *testing.T) {
	cases := map[string][]byte{
		"empty":          {},
		"short":          {0xD0, 0xCF},
		"bad signature":  bytes.Repeat([]byte{0xAB}, 1024),
		"header only":    append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, make([]byte, 504)...),
		"zeroed sectors": make([]byte, 4096),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			// A panic here fails the test; an error return is fine.
			if _, err := cfb.Open(bytes.NewReader(data)); err == nil {
				t.Log("Open accepted the input; acceptable as long as it did not panic")
			}
		})
	}
}

// A corrupt header must not be trusted to size allocations.
func TestOpenRejectsImplausibleSectorSize(t *testing.T) {
	data := make([]byte, 512)
	copy(data, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
	// Sector shift of 63 would imply a 2^63-byte sector.
	data[30], data[31] = 63, 0
	if _, err := cfb.Open(bytes.NewReader(data)); err == nil {
		t.Error("expected an error for an implausible sector shift")
	}
}

// Bit-flipping a real file exercises the parser against structurally
// plausible but corrupt input, which is where index-out-of-range panics hide.
func TestOpenSurvivesCorruptedRealFile(t *testing.T) {
	original, err := os.ReadFile(sampleFile)
	if err != nil {
		t.Skipf("sample file unavailable: %v", err)
	}

	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 200; i++ {
		data := append([]byte(nil), original...)
		// Corrupt a handful of bytes in the header and FAT region, where
		// the structural metadata lives.
		for j := 0; j < 8; j++ {
			data[rng.Intn(8192)] = byte(rng.Intn(256))
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic on corrupted input (iteration %d): %v", i, r)
				}
			}()
			f, err := cfb.Open(bytes.NewReader(data))
			if err != nil {
				return // rejecting corrupt input is the expected outcome
			}
			// If it opened, walking it must also be panic-free.
			walk(f, f.Root, 0)
		}()
	}
}

func walk(f *cfb.File, e *cfb.Entry, depth int) {
	if e == nil || depth > 16 {
		return
	}
	if e.IsStream() {
		_, _ = f.ReadStream(e)
		return
	}
	for _, child := range e.Children {
		walk(f, child, depth+1)
	}
}
