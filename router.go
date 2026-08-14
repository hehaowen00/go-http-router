package gohttprouter

import (
	"fmt"
	"strings"
)

type Router[T any] struct {
	nodes [methodCount][]node[T]
}

func New[T any]() *Router[T] {
	return &Router[T]{}
}

func (r *Router[T]) Add(method string, path string, handler T) error {
	sequence := splitPath(path)

	err := validateSeq(sequence)
	if err != nil {
		return fmt.Errorf("invalid path - %w", err)
	}

	m := methodToEnum(method)
	if m == methodNotFound {
		return fmt.Errorf("unsupported method - %s", method)
	}

	if r.nodes[m] == nil {
		r.nodes[m] = make([]node[T], 1, 64)
	}

	insert(&r.nodes[m], 0, sequence, handler)

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

	h := search(r.nodes[m], 0, path, 0, params)
	if h == nil {
		params.reset()
	}

	return h
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

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	if r.nodes[m] == nil {
		return
	}

	remove(r.nodes[m], 0, sequence)
}
