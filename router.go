package gohttprouter

import (
	"fmt"
	"slices"
	"strings"
)

type Router[T any] struct {
	static    [methodCount]map[string]handlerPtr
	staticLen [methodCount]staticLenFilter
	nodes     [methodCount][]node
	handlers  [methodCount][]T
}

func New[T any]() *Router[T] {
	return &Router[T]{}
}

func (r *Router[T]) Add(method string, path string, handler T) error {
	m := methodToEnum(method)
	if m == methodNotFound {
		return fmt.Errorf("unsupported method - %s", method)
	}

	if !strings.ContainsAny(path, ":*") {
		key := normalizeStaticPath(path)
		r.staticLen[m].set(len(key))

		idx := handlerPtr(len(r.handlers[m]))
		r.handlers[m] = append(r.handlers[m], handler)
		if r.static[m] == nil {
			r.static[m] = make(map[string]handlerPtr)
		}

		r.static[m][key] = idx
		return nil
	}

	sequence := splitPath(path)

	err := validateSeq(sequence)
	if err != nil {
		return fmt.Errorf("invalid path - %w", err)
	}

	if len(r.nodes[m])+2*len(sequence)+2 > maxTreeNodes {
		return fmt.Errorf(
			"too many param routes - tree limit %d nodes",
			maxTreeNodes,
		)
	}

	if r.nodes[m] == nil {
		r.nodes[m] = make([]node, 1, 64)
		r.nodes[m][0].handlerIdx = -1
		r.nodes[m][0].slashChild = -1
	}

	idx := handlerPtr(len(r.handlers[m]))
	r.handlers[m] = append(r.handlers[m], handler)

	insert(&r.nodes[m], 0, sequence, idx)

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

	if r.staticLen[m].count > 0 {
		if r.staticLen[m].has(len(key)) {
			if static := r.static[m]; static != nil {
				if idx, ok := static[key]; ok {
					return r.handlerAt(m, idx)
				}
			}
		}
	}

	if len(r.nodes[m]) == 0 {
		return nil
	}

	idx := search(r.nodes[m], path, params)
	if idx < 0 {
		params.reset()
		return nil
	}

	return r.handlerAt(m, idx)
}

func (r *Router[T]) handlerAt(m methodEnum, idx handlerPtr) *T {
	return &r.handlers[m][idx]
}

func (r *Router[T]) refreshStaticLenSet(m methodEnum) {
	r.staticLen[m] = staticLenFilter{}

	if t := r.static[m]; t != nil {
		for key := range t {
			r.staticLen[m].set(len(key))
		}
	}
}

func (r *Router[T]) Remove(method string, path string) {
	m := methodToEnum(method)
	if m == methodNotFound {
		return
	}

	if !strings.ContainsAny(path, ":*") {
		if t := r.static[m]; t != nil {
			key := normalizeStaticPath(path)
			if idx, ok := t[key]; ok {
				delete(t, key)
				r.removeHandler(m, idx)

				if len(t) == 0 {
					r.static[m] = nil
				}

				r.refreshStaticLenSet(m)
			}
		}

		return
	}

	sequence := splitPath(path)

	err := validateSeq(sequence)
	if err != nil {
		panic(err)
	}

	if r.nodes[m] == nil {
		return
	}

	if remove(r.nodes[m], 0, sequence) {
		r.nodes[m] = compactNodes(r.nodes[m])
	}
}

func (r *Router[T]) removeHandler(m methodEnum, removed handlerPtr) {
	handlers := r.handlers[m]
	n := len(handlers)

	if int(removed) < 0 || int(removed) >= n {
		return
	}

	if int(removed) == n-1 {
		var zero T
		handlers[n-1] = zero
		r.handlers[m] = handlers[:n-1]
		return
	}

	r.handlers[m] = slices.Delete(handlers, int(removed), int(removed)+1)

	if t := r.static[m]; t != nil {
		for key, idx := range t {
			if idx > removed {
				t[key] = idx - 1
			}
		}
	}

	for i := range r.nodes[m] {
		if r.nodes[m][i].handlerIdx > removed {
			r.nodes[m][i].handlerIdx--
		}
	}
}
