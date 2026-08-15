//go:build amd64 || arm64

package gohttprouter

import (
	"math/bits"
	"unsafe"
)

const (
	slashWord = 0x2f2f2f2f2f2f2f2f
	loWord    = 0x0101010101010101
	hiWord    = 0x8080808080808080
)

func nextSlash(s string, i, end int) int {
	sd := unsafe.StringData(s)

	for i+8 <= end {
		w := *(*uint64)(unsafe.Add(unsafe.Pointer(sd), i))

		x := w ^ slashWord
		m := (x - loWord) &^ x & hiWord

		if m != 0 {
			return i + bits.TrailingZeros64(m)>>3
		}

		i += 8
	}

	for i < end {
		if s[i] == '/' {
			return i
		}

		i++
	}

	return end
}
