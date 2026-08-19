package gohttprouter

import (
	"unsafe"
)

func (r *Router[T]) MemSize() uintptr {
	size := unsafe.Sizeof(*r)

	for m := methodGet; m < methodCount; m++ {
		size += staticMapMemSize(r.static[m])

		nodes := r.nodes[m]
		size += uintptr(cap(nodes)) * unsafe.Sizeof(node{})

		for i := range nodes {
			n := &nodes[i]

			size += uintptr(len(n.prefix))
			size += uintptr(len(n.catchAllName))
			size += uintptr(cap(n.children)) * unsafe.Sizeof(childRef{})
			size += uintptr(cap(n.wildcard)) * unsafe.Sizeof(wildcard{})

			for j := range n.wildcard {
				w := &n.wildcard[j]

				size += uintptr(cap(w.params)) * unsafe.Sizeof("")

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

func staticMapMemSize(t map[string]handlerPtr) uintptr {
	if t == nil {
		return 0
	}

	size := unsafe.Sizeof(t)

	slotSize := unsafe.Sizeof(uint8(0)) + unsafe.Sizeof("") + unsafe.Sizeof(handlerPtr(0))
	slots := 8
	for slots < len(t) {
		slots *= 2
	}
	size += uintptr(slots) * slotSize

	for key := range t {
		size += uintptr(len(key))
	}

	return size
}
