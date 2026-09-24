package gohttprouter

import "unsafe"

// word and last are the key's first and last 8 bytes. Together they cover
// every byte of a key up to 16 long, so those keys never need the full string
// compare.
type staticEntry struct {
	word uint64
	last uint64
	key  string
	idx  handlerPtr
}

// Entries are sorted by key length, then by (word, last, key) within a length. off[n] is the index of the first entry
// with a key of length n or more, so keys shorter than staticLenBits find
// their bucket with two loads instead of a binary search. Longer keys all sit
// from off[staticLenBits] onwards and fall back to lowerBound.
type staticTable struct {
	entries []staticEntry
	off     [staticLenBits + 1]uint16
}

const maxStaticRoutes = 1<<16 - 1

// shiftOffsets adjusts off after an entry of length n is inserted (d = 1)
// or removed (d = -1): only buckets for longer keys move.
func (t *staticTable) shiftOffsets(n int, d int) {
	for k := n + 1; k < len(t.off); k++ {
		t.off[k] = uint16(int(t.off[k]) + d)
	}
}

// bucket returns the entry range to scan for a key of length n. For long
// keys the range runs to the end of the table and scan stops at the first
// entry of another length.
func (t *staticTable) bucket(n int) (int, int) {
	if n < staticLenBits {
		return int(t.off[n]), int(t.off[n+1])
	}

	return t.lowerBound(n), len(t.entries)
}

// Buckets up to this size are scanned linearly; larger ones are binary
// searched on (word, last). Keys of staticLenBits or more always scan
// linearly, since bucket does not give their exact end.
const staticLinearMax = 8

func (e *staticEntry) before(w, lw uint64) bool {
	return e.word < w || (e.word == w && e.last < lw)
}

func (t *staticTable) scan(key string, lo, hi int) (handlerPtr, bool) {
	n := len(key)
	w := keyWord(key)

	if hi-lo > staticLinearMax && n < staticLenBits {
		return t.search(key, w, lo, hi)
	}

	for i := lo; i < hi; i++ {
		e := &t.entries[i]

		if len(e.key) != n {
			break
		}

		if e.word != w {
			continue
		}

		if n <= 16 {
			if e.last == lastWord(key) {
				return e.idx, true
			}

			continue
		}

		if e.key == key {
			return e.idx, true
		}
	}

	return 0, false
}

// search binary searches the bucket [lo, hi) for key. Keys up to 16 bytes
// are unique by (word, last); longer keys that share both are compared in
// full.
func (t *staticTable) search(key string, w uint64, lo, hi int) (handlerPtr, bool) {
	lw := lastWord(key)

	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if t.entries[mid].before(w, lw) {
			lo = mid + 1
		} else {
			hi = mid
		}
	}

	for i := lo; i < len(t.entries); i++ {
		e := &t.entries[i]

		if e.word != w || e.last != lw || len(e.key) != len(key) {
			break
		}

		if len(key) <= 16 || e.key == key {
			return e.idx, true
		}
	}

	return 0, false
}

func keyWord(s string) uint64 {
	if len(s) >= 8 {
		return *(*uint64)(unsafe.Pointer(unsafe.StringData(s)))
	}

	var w uint64
	for i := 0; i < len(s); i++ {
		w |= uint64(s[i]) << (8 * i)
	}

	return w
}

// lastWord is the key's last 8 bytes, or 0 for keys shorter than 8, whose
// first word already covers them.
func lastWord(s string) uint64 {
	if len(s) < 8 {
		return 0
	}

	return *(*uint64)(unsafe.Add(unsafe.Pointer(unsafe.StringData(s)), len(s)-8))
}

func (t *staticTable) lowerBound(n int) int {
	lo, hi := 0, len(t.entries)

	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if len(t.entries[mid].key) < n {
			lo = mid + 1
		} else {
			hi = mid
		}
	}

	return lo
}

func (t *staticTable) get(key string) (handlerPtr, bool) {
	lo, hi := t.bucket(len(key))
	return t.scan(key, lo, hi)
}

func (t *staticTable) set(key string, idx handlerPtr) {
	lo, hi := t.lowerBound(len(key)), t.lowerBound(len(key)+1)
	w, lw := keyWord(key), lastWord(key)

	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if t.entries[mid].before(w, lw) {
			lo = mid + 1
		} else {
			hi = mid
		}
	}

	// Entries sharing (word, last) are ordered by key.
	i := lo
	for ; i < len(t.entries); i++ {
		e := &t.entries[i]
		if len(e.key) != len(key) || e.word != w || e.last != lw || e.key > key {
			break
		}

		if e.key == key {
			e.idx = idx
			return
		}
	}

	t.entries = append(t.entries, staticEntry{})
	copy(t.entries[i+1:], t.entries[i:])
	t.entries[i] = staticEntry{
		word: keyWord(key),
		last: lastWord(key),
		key:  key,
		idx:  idx,
	}
	t.shiftOffsets(len(key), 1)
}

func (t *staticTable) remove(key string) (handlerPtr, bool) {
	for i := t.lowerBound(len(key)); i < len(t.entries); i++ {
		if len(t.entries[i].key) != len(key) {
			break
		}

		if t.entries[i].key == key {
			idx := t.entries[i].idx
			t.entries = append(t.entries[:i], t.entries[i+1:]...)
			t.shiftOffsets(len(key), -1)

			return idx, true
		}
	}

	return 0, false
}

func (t *staticTable) len() int {
	return len(t.entries)
}
