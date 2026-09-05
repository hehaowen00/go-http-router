package gohttprouter

import "unsafe"

type staticEntry struct {
	word uint64
	key  string
	idx  handlerPtr
}

type staticTable struct {
	entries []staticEntry
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
	n := len(key)
	w := keyWord(key)

	for i := t.lowerBound(n); i < len(t.entries); i++ {
		e := &t.entries[i]

		if len(e.key) != n {
			break
		}

		if e.word == w && e.key == key {
			return e.idx, true
		}
	}

	return 0, false
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
	t.entries[i] = staticEntry{word: keyWord(key), key: key, idx: idx}
}

func (t *staticTable) remove(key string) (handlerPtr, bool) {
	for i := t.lowerBound(len(key)); i < len(t.entries); i++ {
		if len(t.entries[i].key) != len(key) {
			break
		}

		if t.entries[i].key == key {
			idx := t.entries[i].idx
			t.entries = append(t.entries[:i], t.entries[i+1:]...)

			return idx, true
		}
	}

	return 0, false
}

func (t *staticTable) len() int {
	return len(t.entries)
}
