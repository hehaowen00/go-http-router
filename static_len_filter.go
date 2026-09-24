package gohttprouter

const staticLenBits = 256

// staticLenFilter rejects most param paths before the static table is
// scanned, including ones that share a length with a static route.
type staticLenFilter struct {
	tail [staticLenBits / 64]uint64
}

// tailBit folds the key's length and last byte into one bit.
func tailBit(key string) uint {
	n := len(key)
	return (uint(n)*31 + uint(key[n-1])) & (staticLenBits - 1)
}

func (s *staticLenFilter) setTail(key string) {
	b := tailBit(key)
	s.tail[b>>6] |= 1 << (b & 63)
}

func (s *staticLenFilter) hasTail(key string) bool {
	b := tailBit(key)
	return s.tail[b>>6]&(1<<(b&63)) != 0
}
