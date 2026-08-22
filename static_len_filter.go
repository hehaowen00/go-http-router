package gohttprouter

const (
	staticLenShift = 11
	staticLenBits  = 1 << staticLenShift
)

// golden ratio constant
func staticLenHash(n int) uint64 {
	return (uint64(n) * 0x9E3779B97F4A7C15) >> (64 - staticLenShift)
}

// bloom filter like construct for checking if a path of a certain length is
// a member of the router static map
type staticLenFilter struct {
	bits  [staticLenBits / 64]uint64
	count int
}

func (s *staticLenFilter) set(n int) {
	h := staticLenHash(n)
	s.bits[h>>6] |= 1 << (h & 63)
	s.count++
}

func (s *staticLenFilter) has(n int) bool {
	h := staticLenHash(n)
	return s.bits[h>>6]&(1<<(h&63)) != 0
}
