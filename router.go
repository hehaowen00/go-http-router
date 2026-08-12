package gohttprouter

import (
	"net/http"
	"slices"
	"strings"
)

type Router struct {
	nodes []node
	roots [methodCount]nodePtr
}

func New() *Router {
	r := &Router{
		nodes: make([]node, 0, 64),
	}

	for i := range r.roots {
		r.roots[i] = r.newNode()
	}

	return r
}

func (r *Router) Add(method string, path string, handler Handler) {
	pathSeq := splitPath(path)

	err := validateSeq(pathSeq)
	if err != nil {
		panic(err)
	}

	methodIndex := methodToEnum(method)
	if methodIndex == 255 {
		return
	}

	r.insert(r.roots[methodIndex], pathSeq, handler)
}

func (r *Router) Search(method string, path string, params *Params) Handler {
	params.reset()

	methodIndex := methodToEnum(method)
	if methodIndex == 255 {
		return nil
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	h := r.search(r.roots[methodIndex], path, 0, params)
	if h == nil {
		params.reset()
	}

	return h
}

func (r *Router) Remove(method string, path string) {
	pathSeq := splitPath(path)

	err := validateSeq(pathSeq)
	if err != nil {
		panic(err)
	}

	methodIndex := methodToEnum(method)
	if methodIndex == 255 {
		return
	}

	r.remove(r.roots[methodIndex], pathSeq)
}

func (r *Router) remove(nodeIdx nodePtr, pathSeq []string) bool {
	if len(pathSeq) == 0 {
		n := &r.nodes[nodeIdx]

		if n.handler == nil {
			return false
		}

		n.handler = nil
		return true
	}

	currentSegment := pathSeq[0]
	n := &r.nodes[nodeIdx]

	if isParam(currentSegment) {
		name := paramName(currentSegment)

		for i := range n.wildcard {
			if n.wildcard[i].name != name {
				continue
			}

			childIdx := n.wildcard[i].node

			removed := r.remove(childIdx, pathSeq[1:])
			if !removed {
				return false
			}

			n = &r.nodes[nodeIdx]
			if r.nodes[childIdx].isEmpty() {
				n.wildcard = slices.Delete(n.wildcard, i, i+1)
			}
			n.hasParams = n.recomputeHasParams(r)

			return true
		}

		return false
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
		return false
	}

	childIdx := n.children[closestIdx]

	if len(r.nodes[childIdx].prefix) > best {
		return false
	}

	if best < len(currentSegment) {
		pathSeq[0] = currentSegment[best:]
	} else {
		pathSeq = slices.Delete(pathSeq, 0, 1)
	}

	removed := r.remove(childIdx, pathSeq)
	if !removed {
		return false
	}

	n = &r.nodes[nodeIdx]
	if r.nodes[childIdx].isEmpty() {
		n.children = slices.Delete(n.children, closestIdx, closestIdx+1)
		n.rebuildFingerprint(r)
	}
	n.hasParams = n.recomputeHasParams(r)

	return true
}

func (r *Router) GET(path string, handler Handler) {
	r.Add(http.MethodGet, path, handler)
}

func (r *Router) QUERY(path string, handler Handler) {
	r.Add("QUERY", path, handler)
}

func (r *Router) POST(path string, handler Handler) {
	r.Add(http.MethodPost, path, handler)
}

func (r *Router) PUT(path string, handler Handler) {
	r.Add(http.MethodPut, path, handler)
}

func (r *Router) DELETE(path string, handler Handler) {
	r.Add(http.MethodDelete, path, handler)
}

func (r *Router) CONNECT(path string, handler Handler) {
	r.Add(http.MethodConnect, path, handler)
}

func (r *Router) OPTIONS(path string, handler Handler) {
	r.Add(http.MethodOptions, path, handler)
}

func (r *Router) HEAD(path string, handler Handler) {
	r.Add(http.MethodHead, path, handler)
}
