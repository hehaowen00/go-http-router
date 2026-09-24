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

// Entries are sorted by key length. off[n] is the index of the first entry
// with a key of length n or more, so keys shorter than staticLenBits find
// their bucket with two loads instead of a binary search. Longer keys all sit
// from off[staticLenBits] onwards and fall back to lowerBound.
type staticTable struct {
	entries []staticEntry
	off     [staticLenBits + 1]uint16
}

const maxStaticRoutes = 1<<16 - 1

func (t *staticTable) rebuildOffsets() {
	i := 0

	for n := range t.off {
		for i < len(t.entries) && len(t.entries[i].key) < n {
			i++
		}

		t.off[n] = uint16(i)
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

func (t *staticTable) scan(key string, lo, hi int) (handlerPtr, bool) {
	n := len(key)
	w := keyWord(key)

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
	i := t.lowerBound(len(key))

	for ; i < len(t.entries) && len(t.entries[i].key) == len(key); i++ {
		if t.entries[i].key == key {
			t.entries[i].idx = idx
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
	t.rebuildOffsets()
}

func (t *staticTable) remove(key string) (handlerPtr, bool) {
	for i := t.lowerBound(len(key)); i < len(t.entries); i++ {
		if len(t.entries[i].key) != len(key) {
			break
		}

		if t.entries[i].key == key {
			idx := t.entries[i].idx
			t.entries = append(t.entries[:i], t.entries[i+1:]...)
			t.rebuildOffsets()

			return idx, true
		}
	}

	return 0, false
}

func (t *staticTable) len() int {
	return len(t.entries)
}
