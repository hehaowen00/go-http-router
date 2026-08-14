package gohttprouter

import (
	"fmt"
	"strings"
)

type Router[T any] struct {
	static   [methodCount]map[string]handlerPtr
	nodes    [methodCount][]node
	handlers [methodCount][]T
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

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	if static := r.static[m]; static != nil {
		if idx, ok := static[staticKey(path)]; ok {
			return r.handlerAt(m, idx)
		}
	}

	if r.nodes[m] == nil {
		return nil
	}

	idx := search(&r.nodes[m][0], r.nodes[m], path, 0, params)
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
			delete(r.static[m], normalizeStaticPath(path))
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
