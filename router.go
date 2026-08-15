package gohttprouter

import (
	"fmt"
	"slices"
	"strings"
)

type Router[T any] struct {
	static       [methodCount]map[string]handlerPtr
	staticMaxLen [methodCount]int
	nodes        [methodCount][]node
	handlers     [methodCount][]T
}

func New[T any]() *Router[T] {
	return &Router[T]{
		static: [methodCount]map[string]handlerPtr{},
	}
}

func (r *Router[T]) Add(method string, path string, handler T) error {
	m := methodToEnum(method)
	if m == methodNotFound {
		return fmt.Errorf("unsupported method - %s", method)
	}

	if !strings.ContainsAny(path, ":*") {
		key := normalizeStaticPath(path)
		if len(key) > r.staticMaxLen[m] {
			r.staticMaxLen[m] = len(key)
		}

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

	if key := staticKey(path); len(key) <= r.staticMaxLen[m] {
		if static := r.static[m]; static != nil {
			if idx, ok := static[key]; ok {
				return r.handlerAt(m, idx)
			}
		}
	}

	if r.nodes[m] == nil {
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

func (r *Router[T]) Remove(method string, path string) {
	m := methodToEnum(method)
	if m == methodNotFound {
		return
	}

	if !strings.ContainsAny(path, ":*") {
		if r.static[m] != nil {
			key := normalizeStaticPath(path)
			if idx, ok := r.static[m][key]; ok {
				delete(r.static[m], key)
				r.removeHandler(m, idx)
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

	remove(r.nodes[m], 0, sequence)
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

	for key, idx := range r.static[m] {
		if idx > removed {
			r.static[m][key] = idx - 1
		}
	}

	for i := range r.nodes[m] {
		if r.nodes[m][i].handlerIdx > removed {
			r.nodes[m][i].handlerIdx--
		}
	}
}
