package gohttprouter

import (
	"fmt"
	"strings"
)

type Router[T any] struct {
	static   [methodCount]map[string]handlerPtr
	nodes    [methodCount][]node[T]
	handlers [methodCount][]T
}

func New[T any]() *Router[T] {
	return &Router[T]{
		static: [methodCount]map[string]handlerPtr{},
	}
}

func (r *Router[T]) Add(method string, path string, handler T) error {
	sequence := splitPath(path)

	m := methodToEnum(method)
	if m == methodNotFound {
		return fmt.Errorf("unsupported method - %s", method)
	}

	err := validateSeq(sequence)
	if err != nil {
		return fmt.Errorf("invalid path - %w", err)
	}

	if r.nodes[m] == nil {
		r.nodes[m] = make([]node[T], 1, 64)
		r.nodes[m][0].handlerIdx = -1
	}

	idx := handlerPtr(len(r.handlers[m]))
	r.handlers[m] = append(r.handlers[m], handler)

	if isStatic := isStaticSequence(sequence); isStatic {
		if r.static[m] == nil {
			r.static[m] = make(map[string]handlerPtr)
		}

		r.static[m][staticKey(sequence[0])] = idx

		return nil
	}

	insert(&r.nodes[m], 0, sequence, idx)

	return nil
}

func (r *Router[T]) Search(method string, path string, params *Params) *T {
	params.reset()

	m := methodToEnum(method)
	if m == methodNotFound {
		return nil
	}

	if r.nodes[m] == nil {
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
	sequence := splitPath(path)

	err := validateSeq(sequence)
	if err != nil {
		panic(err)
	}

	m := methodToEnum(method)
	if m == methodNotFound {
		return
	}

	if isStaticSequence(sequence) && r.static[m] != nil {
		delete(r.static[m], staticKey(sequence[0]))
		return
	}

	if r.nodes[m] == nil {
		return
	}

	remove(r.nodes[m], 0, sequence)
}
