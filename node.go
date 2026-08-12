package gohttprouter

import (
	"net/http"
	"slices"
)

type node struct {
	prefix   string
	handlers methodHandler
	children []int32
	wildcard []wildcard
}

type wildcard struct {
	name string
	node int32
}

func (r *Router) newNode() int32 {
	r.nodes = append(r.nodes, node{})
	return int32(len(r.nodes) - 1)
}

func (r *Router) getWildcard(idx int32, name string) int32 {
	n := &r.nodes[idx]

	for i := range n.wildcard {
		if n.wildcard[i].name == name {
			return int32(i)
		}
	}

	childIdx := r.newNode()
	n = &r.nodes[idx]

	n.wildcard = append(n.wildcard, wildcard{
		name: name,
		node: childIdx,
	})

	return int32(len(n.wildcard) - 1)
}

func (r *Router) search(
	idx int32,
	method string,
	path string,
	i int,
	params *Params,
) Handler {
	n := &r.nodes[idx]

	if i == len(path) || (i == len(path)-1 && path[i] == '/') {
		if h := n.handlers.Get(method); h != nil {
			return h
		}

		for _, c := range n.children {
			if r.nodes[c].prefix == "/" {
				return r.nodes[c].handlers.Get(method)
			}
		}

		return nil
	}

	if i < len(path) {
		b := path[i]
		rem := len(path) - i

		for _, c := range n.children {
			child := &r.nodes[c]
			if b == child.prefix[0] && hasPrefixAt(path, i, child.prefix) {
				h := r.search(c, method, path, i+len(child.prefix), params)
				if h != nil {
					return h
				}

				break
			}

			if len(child.prefix) > rem && leafMatch(path[i:], child.prefix) {
				return child.handlers.Get(method)
			}
		}
	}

	if len(n.wildcard) == 0 {
		return nil
	}

	segmentStart := i
	if path[segmentStart] == '/' {
		segmentStart++
	}

	segmentEnd := -1
	for j := segmentStart; j < len(path); j++ {
		if path[j] == '/' {
			segmentEnd = j - segmentStart
			break
		}
	}
	if segmentEnd == -1 {
		segmentEnd = len(path) - segmentStart
	}

	value := path[segmentStart : segmentStart+segmentEnd]

	for wi := range n.wildcard {
		wc := &n.wildcard[wi]
		params.set(wc.name, value)

		h := r.search(wc.node, method, path, segmentStart+segmentEnd, params)
		if h != nil {
			return h
		}
	}

	return nil
}

func (r *Router) insert(
	idx int32,
	method string,
	pathSeq []string,
	handler Handler,
) {
	if len(pathSeq) == 0 {
		r.nodes[idx].handlers.Insert(method, handler)
		return
	}

	currentSegment := pathSeq[0]
	n := &r.nodes[idx]

	if isParam(currentSegment) {
		name := paramName(currentSegment)

		wildcardIdx := r.getWildcard(idx, name)
		n = &r.nodes[idx]

		pathSeq = slices.Delete(pathSeq, 0, 1)
		r.insert(n.wildcard[wildcardIdx].node, method, pathSeq, handler)

		return
	}

	closestIdx := -1
	best := 0

	for i := range n.children {
		score := longestMatch(currentSegment, r.nodes[n.children[i]].prefix)
		if score > best {
			best = score
			closestIdx = i
		}
	}

	if closestIdx < 0 {
		childIdx := r.newNode()
		r.nodes[childIdx].prefix = currentSegment
		r.insert(childIdx, method, slices.Delete(pathSeq, 0, 1), handler)

		n = &r.nodes[idx]
		n.children = append(n.children, childIdx)

		return
	}

	closest := &r.nodes[n.children[closestIdx]]
	if len(closest.prefix) == best {
		if best == len(pathSeq[0]) {
			pathSeq = slices.Delete(pathSeq, 0, 1)
		} else {
			pathSeq[0] = pathSeq[0][best:]
		}

		r.insert(n.children[closestIdx], method, pathSeq, handler)
		return
	}

	if len(closest.prefix) > best {
		newChildIdx := r.newNode()
		n = &r.nodes[idx]
		closest = &r.nodes[n.children[closestIdx]]

		newChild := &r.nodes[newChildIdx]
		newChild.prefix = closest.prefix[:best]
		closest.prefix = closest.prefix[best:]
		newChild.children = append(newChild.children, n.children[closestIdx])

		n.children[closestIdx] = newChildIdx

		if best >= len(currentSegment) {
			pathSeq = slices.Delete(pathSeq, 0, 1)
		} else {
			pathSeq[0] = pathSeq[0][best:]
		}

		r.insert(newChildIdx, method, pathSeq, handler)
	}
}

type methodHandler struct {
	get     Handler
	query   Handler
	post    Handler
	patch   Handler
	put     Handler
	delete  Handler
	connect Handler
	options Handler
	head    Handler
	count   int
}

func (h *methodHandler) Len() int {
	return h.count
}

func (h *methodHandler) Get(method string) Handler {
	switch method {
	case http.MethodGet:
		return h.get
	case http.MethodPost:
		return h.post
	case http.MethodPatch:
		return h.patch
	case http.MethodPut:
		return h.put
	case http.MethodDelete:
		return h.delete
	case http.MethodConnect:
		return h.connect
	case http.MethodOptions:
		return h.options
	case http.MethodHead:
		return h.head
	default:
		return nil
	}
}

func (h *methodHandler) Insert(method string, handler Handler) {
	h.set(method, handler)
}

func (h *methodHandler) Remove(method string) bool {
	return h.set(method, nil)
}

func (h *methodHandler) set(method string, handler Handler) bool {
	var target *Handler

	switch method {
	case http.MethodGet:
		target = &h.get
	case http.MethodPost:
		target = &h.post
	case http.MethodPatch:
		target = &h.patch
	case http.MethodPut:
		target = &h.put
	case http.MethodDelete:
		target = &h.delete
	case http.MethodConnect:
		target = &h.connect
	case http.MethodOptions:
		target = &h.options
	case http.MethodHead:
		target = &h.head
	default:
		return false
	}

	if *target != nil && handler == nil {
		*target = handler
		h.count--
		return true
	} else if *target == nil && handler != nil {
		*target = handler
		h.count++
		return true
	}

	return false
}
