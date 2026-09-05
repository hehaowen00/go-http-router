package gohttprouter

import (
	"fmt"
	"slices"
	"strings"
)

type Router[T any] struct {
	static    [methodCount]staticTable
	staticLen [methodCount]staticLenFilter
	nodes     []node
	roots     [methodCount]nodePtr
	handlers  []T
}

func New[T any]() *Router[T] {
	r := &Router[T]{}
	for i := range r.roots {
		r.roots[i] = -1
	}

	return r
}

func (r *Router[T]) Add(method string, path string, handler T) error {
	m := methodToEnum(method)
	if m == methodNotFound {
		return fmt.Errorf("unsupported method - %s", method)
	}

	if !strings.ContainsAny(path, ":*") {
		key := normalizeStaticPath(path)
		r.staticLen[m].set(len(key))

		idx := handlerPtr(len(r.handlers))
		r.handlers = append(r.handlers, handler)
		r.static[m].set(key, idx)

		return nil
	}

	sequence := splitPath(path)

	err := validateSeq(sequence)
	if err != nil {
		return fmt.Errorf("invalid path - %w", err)
	}

	// Reserve every name up front so the ids are guaranteed by the time
	// insert reaches for them.
	for _, seg := range sequence {
		var name string

		switch {
		case isParam(seg):
			name = paramName(seg)
		case isCatchAll(seg):
			name = catchAllName(seg)
		default:
			continue
		}

		if _, ok := internName(name); !ok {
			return fmt.Errorf(
				"too many distinct param names - limit %d",
				maxParamNames,
			)
		}
	}

	if len(r.nodes)+2*len(sequence)+2 > maxTreeNodes {
		return fmt.Errorf(
			"too many param routes - tree limit %d nodes",
			maxTreeNodes,
		)
	}

	if r.nodes == nil {
		r.nodes = make([]node, 0, 64)
	}

	if r.roots[m] < 0 {
		r.roots[m] = newNode(&r.nodes)
	}

	idx := handlerPtr(len(r.handlers))
	r.handlers = append(r.handlers, handler)

	insert(&r.nodes, r.roots[m], sequence, idx)

	return nil
}

func (r *Router[T]) Search(method string, path string, params *Params) *T {
	params.reset()

	m := methodToEnum(method)
	if m == methodNotFound {
		return nil
	}

	if len(path) == 0 {
		path = "/"
	} else if path[0] != '/' {
		path = "/" + path
	}

	key := staticKey(path)

	if r.staticLen[m].has(len(key)) {
		if idx, ok := r.static[m].get(key); ok {
			return r.handlerAt(m, idx)
		}
	}

	root := r.roots[m]
	if root < 0 {
		return nil
	}

	idx := search(r.nodes, root, path, params)
	if idx < 0 {
		params.reset()
		return nil
	}

	return r.handlerAt(m, idx)
}

func (r *Router[T]) Compact() {
	if cap(r.nodes) > len(r.nodes) {
		nodes := make([]node, len(r.nodes))
		copy(nodes, r.nodes)
		r.nodes = nodes
	}

	if cap(r.handlers) > len(r.handlers) {
		handlers := make([]T, len(r.handlers))
		copy(handlers, r.handlers)
		r.handlers = handlers
	}
}

func (r *Router[T]) handlerAt(m methodEnum, idx handlerPtr) *T {
	_ = m

	return &r.handlers[idx]
}

func (r *Router[T]) refreshStaticLenSet(m methodEnum) {
	r.staticLen[m] = staticLenFilter{}

	for i := range r.static[m].entries {
		r.staticLen[m].set(len(r.static[m].entries[i].key))
	}
}

func (r *Router[T]) Remove(method string, path string) {
	m := methodToEnum(method)
	if m == methodNotFound {
		return
	}

	if !strings.ContainsAny(path, ":*") {
		key := normalizeStaticPath(path)
		if idx, ok := r.static[m].remove(key); ok {
			r.removeHandler(m, idx)
			r.refreshStaticLenSet(m)
		}

		return
	}

	sequence := splitPath(path)

	err := validateSeq(sequence)
	if err != nil {
		panic(err)
	}

	if r.roots[m] < 0 {
		return
	}

	if remove(r.nodes, r.roots[m], sequence) {
		r.nodes = compactNodes(r.nodes, &r.roots)
	}
}

func (r *Router[T]) removeHandler(m methodEnum, removed handlerPtr) {
	_ = m

	handlers := r.handlers
	n := len(handlers)

	if int(removed) < 0 || int(removed) >= n {
		return
	}

	if int(removed) == n-1 {
		var zero T
		handlers[n-1] = zero
		r.handlers = handlers[:n-1]
		return
	}

	r.handlers = slices.Delete(handlers, int(removed), int(removed)+1)

	for mi := range r.static {
		es := r.static[mi].entries
		for i := range es {
			if es[i].idx > removed {
				es[i].idx--
			}
		}
	}

	for i := range r.nodes {
		if r.nodes[i].handlerIdx > removed {
			r.nodes[i].handlerIdx--
		}
	}
}
