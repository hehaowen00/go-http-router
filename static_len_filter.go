package gohttprouter

const staticLenBits = 256

type staticLenFilter struct {
	bits [staticLenBits / 64]uint64
	tail [staticLenBits / 64]uint64
	long bool
}

func (s *staticLenFilter) set(n int) {
	if n >= staticLenBits {
		s.long = true
		return
	}

	s.bits[n>>6] |= 1 << (uint(n) & 63)
}

func (s *staticLenFilter) has(n int) bool {
	if n >= staticLenBits {
		return s.long
	}

	return s.bits[n>>6]&(1<<(uint(n)&63)) != 0
}

// tailBit folds the key's length and last byte into one bit, so param paths
// that share a length with a static route are still mostly rejected before
// the table probe.
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
