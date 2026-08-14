package gohttprouter

import "strings"

type Router[T any] struct {
	nodes [methodCount][]node[T]
}

func New[T any]() *Router[T] {
	return &Router[T]{}
}

func (r *Router[T]) Add(method string, path string, handler T) {
	pathSeq := splitPath(path)

	err := validateSeq(pathSeq)
	if err != nil {
		panic(err)
	}

	m := methodToEnum(method)
	if m == methodNotFound {
		return
	}

	if r.nodes[m] == nil {
		r.nodes[m] = make([]node[T], 1)
	}

	insert(&r.nodes[m], 0, pathSeq, handler)
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
	pathSeq := splitPath(path)

	err := validateSeq(pathSeq)
	if err != nil {
		panic(err)
	}

	m := methodToEnum(method)
	if m == methodNotFound {
		return
	}

	if r.nodes[m] == nil {
		return
	}

	remove(r.nodes[m], 0, pathSeq)
}
