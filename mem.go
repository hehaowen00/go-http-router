package gohttprouter

import (
	"unsafe"
)

func (r *Router[T]) MemSize() uintptr {
	size := unsafe.Sizeof(*r)

	for m := methodGet; m < methodCount; m++ {
		size += staticTableMemSize(&r.static[m])
	}

	{
		nodes := r.nodes
		size += uintptr(cap(nodes)) * unsafe.Sizeof(node{})

		for i := range nodes {
			n := &nodes[i]

			size += uintptr(len(n.prefix))
			size += uintptr(cap(n.children)) * unsafe.Sizeof(childRef{})

			if n.cold == nil {
				continue
			}

			size += unsafe.Sizeof(nodeCold{})
			size += uintptr(cap(n.cold.wildcard)) * unsafe.Sizeof(wildcard{})

			for j := range n.cold.wildcard {
				w := &n.cold.wildcard[j]

				size += uintptr(cap(w.params)) * unsafe.Sizeof(paramID(0))
			}
		}

		var zero T
		size += uintptr(cap(r.handlers)) * unsafe.Sizeof(zero)
	}

	return size
}

func staticTableMemSize(t *staticTable) uintptr {
	size := uintptr(cap(t.entries)) * unsafe.Sizeof(staticEntry{})

	for i := range t.entries {
		size += uintptr(len(t.entries[i].key))
	}

	return size
}
