package gohttprouter

import (
	"net/http"
	"slices"
	"strings"
)

type node struct {
	prefix   string
	handlers methodHandler
	children []*node
	wildcard []*wildcard
	param    string
}

type wildcard struct {
	name string
	node *node
}

func newNode() *node {
	return &node{}
}

func (n *node) Search(method string, path string, i int, params *Params) Handler {
	if i == len(path) || (i == len(path)-1 && path[i] == '/') {
		if h := n.handlers.Get(method); h != nil {
			return h
		}

		for _, child := range n.children {
			if child.prefix == "/" {
				return child.handlers.Get(method)
			}
		}

		return nil
	}

	if i < len(path) {
		b := path[i]

		for _, child := range n.children {
			if b == child.prefix[0] && hasPrefixAt(path, i, child.prefix) {
				h := child.Search(method, path, i+len(child.prefix), params)
				if h != nil {
					return h
				}

				break
			}

			if leafMatch(path[i:], child.prefix) {
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

	idx := strings.IndexAny(path[segmentStart:], "/")
	if idx == -1 {
		idx = len(path) - segmentStart
	}

	value := path[segmentStart : segmentStart+idx]

	for _, wc := range n.wildcard {
		params.set(wc.name, value)

		h := wc.node.Search(method, path, segmentStart+idx, params)
		if h != nil {
			return h
		}
	}

	return nil
}

func (n *node) Insert(method string, pathSeq []string, handler Handler) {
	if len(pathSeq) == 0 {
		n.handlers.Insert(method, handler)
		return
	}

	currentSegment := pathSeq[0]

	if isParam(currentSegment) {
		name := paramName(currentSegment)

		i := n.getWildcard(name)
		pathSeq = slices.Delete(pathSeq, 0, 1)

		wildcardNode := n.wildcard[i]
		wildcardNode.node.Insert(method, pathSeq, handler)

		return
	}

	shortestIdx := -1
	best := 0

	for i := range n.children {
		score := longestMatch(currentSegment, n.children[i].prefix)
		if score > best {
			best = score
			shortestIdx = i
		}
	}

	if shortestIdx < 0 {
		pathSeq = slices.Delete(pathSeq, 0, 1)

		if !isParam(currentSegment) {
			childNode := newNode()
			childNode.prefix = currentSegment
			childNode.Insert(method, pathSeq, handler)
			n.children = append(n.children, childNode)
		} else {
			i := n.getWildcard(currentSegment)
			n.wildcard[i].node.Insert(method, pathSeq, handler)
		}

		return
	}

	closest := n.children[shortestIdx]
	if len(closest.prefix) == best {
		if best == len(pathSeq[0]) {
			pathSeq = slices.Delete(pathSeq, 0, 1)
		} else {
			pathSeq[0] = pathSeq[0][best:]
		}

		closest.Insert(method, pathSeq, handler)
	}

	if len(closest.prefix) > best {
		newChild := newNode()
		newChild.prefix = closest.prefix[:best]
		closest.prefix = closest.prefix[best:]
		newChild.children = append(newChild.children, closest)

		n.children[shortestIdx] = newChild

		if best >= len(currentSegment) {
			pathSeq = slices.Delete(pathSeq, 0, 1)
		} else {
			pathSeq[0] = pathSeq[0][best:]
		}

		newChild.Insert(method, pathSeq, handler)
	}
}

func (n *node) getWildcard(name string) int {
	for i := range n.wildcard {
		if n.wildcard[i].name == name {
			return i
		}
	}

	n.wildcard = append(n.wildcard, &wildcard{
		name: name,
		node: newNode(),
	})

	return len(n.wildcard) - 1
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
