package gohttprouter

import (
	"unsafe"
)

func (r *Router[T]) MemSize() uintptr {
	size := unsafe.Sizeof(*r)

	for m := methodGet; m < methodCount; m++ {
		size += mapMemSize(r.static[m])

		nodes := r.nodes[m]
		size += uintptr(cap(nodes)) * unsafe.Sizeof(node{})

		for i := range nodes {
			n := &nodes[i]

			size += uintptr(len(n.prefix))
			size += uintptr(len(n.catchAllName))
			size += uintptr(cap(n.children)) * unsafe.Sizeof(child{})
			size += uintptr(cap(n.wildcard)) * unsafe.Sizeof(wildcard{})

			for j := range n.wildcard {
				w := &n.wildcard[j]

				var s string
				size += uintptr(cap(w.params)) * unsafe.Sizeof(s)

				for _, p := range w.params {
					size += uintptr(len(p))
				}
			}
		}

		var zero T
		size += uintptr(cap(r.handlers[m])) * unsafe.Sizeof(zero)
	}

	return size
}

type swissMap struct {
	used        uint64
	seed        uintptr
	dirPtr      unsafe.Pointer
	dirLen      int
	globalDepth uint8
	globalShift uint8
	writing     uint8
	tombstone   bool
	clearSeq    uint64
}

type swissTable struct {
	used       uint16
	capacity   uint16
	growthLeft uint16
	localDepth uint8
	index      int
	groups     struct {
		data       unsafe.Pointer
		lengthMask uint64
	}
}

const (
	swissGroupSlots = 8
	maxDirLen       = 1 << 20
)

func swissSlotSize() uintptr {
	return unsafe.Sizeof(struct {
		key  string
		elem handlerPtr
	}{})
}

func swissGroupSize() uintptr {
	return unsafe.Sizeof([swissGroupSlots]uint8{}) +
		swissGroupSlots*swissSlotSize()
}

func mapMemSize(m map[string]handlerPtr) uintptr {
	if m == nil {
		return 0
	}

	h := *(**swissMap)(unsafe.Pointer(&m))
	if h == nil {
		return 0
	}

	size := unsafe.Sizeof(swissMap{})

	switch {
	case h.dirLen == 0:
		if h.dirPtr != nil {
			size += swissGroupSize()
		}

	case h.dirLen > 0 && h.dirPtr != nil && h.dirLen <= maxDirLen:
		size += uintptr(h.dirLen) * unsafe.Sizeof(uintptr(0))

		var seen []unsafe.Pointer
		for _, t := range unsafe.Slice((**swissTable)(h.dirPtr), h.dirLen) {
			if t == nil {
				continue
			}

			dup := false
			for _, s := range seen {
				if s == unsafe.Pointer(t) {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
			seen = append(seen, unsafe.Pointer(t))

			size += unsafe.Sizeof(swissTable{})
			size += uintptr(uint64(t.capacity)/swissGroupSlots) * swissGroupSize()
		}

	default:
		size += uintptr(len(m)) * swissSlotSize() * 8 / 7
	}

	for k := range m {
		size += uintptr(len(k))
	}

	return size
}
