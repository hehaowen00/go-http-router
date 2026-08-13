package gohttprouter

import "strings"

type Router struct {
	nodes [methodCount][]node
}

func New() *Router {
	return &Router{}
}

func (r *Router) Add(method string, path string, handler Handler) {
	pathSeq := splitPath(path)

	err := validateSeq(pathSeq)
	if err != nil {
		panic(err)
	}

	m := methodToEnum(method)
	if m == 255 {
		return
	}

	if r.nodes[m] == nil {
		r.nodes[m] = make([]node, 1)
	}

	insert(&r.nodes[m], 0, pathSeq, handler)
}

func (r *Router) Search(method string, path string, params *Params) Handler {
	params.reset()

	m := methodToEnum(method)
	if m == 255 {
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

func (r *Router) Remove(method string, path string) {
	pathSeq := splitPath(path)

	err := validateSeq(pathSeq)
	if err != nil {
		panic(err)
	}

	m := methodToEnum(method)
	if m == 255 {
		return
	}

	if r.nodes[m] == nil {
		return
	}

	remove(r.nodes[m], 0, pathSeq)
}
