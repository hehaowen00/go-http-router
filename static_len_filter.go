package gohttprouter

const staticLenBits = 256

type staticLenFilter struct {
	bits  [staticLenBits / 64]uint64
	long  bool
	count int32
}

func (s *staticLenFilter) set(n int) {
	s.count++

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
