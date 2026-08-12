package gohttprouter

import (
	"net/http"
)

type Router struct {
	nodes    []node
	handlers []methodHandler
	root     nodePtr
}

func New() *Router {
	r := &Router{
		nodes: make([]node, 0, 64),
	}

	r.root = r.newNode()

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

	r.insert(r.root, methodIndex, pathSeq, handler)
}

func (r *Router) Search(method string, path string, params *Params) Handler {
	params.reset()

	methodIndex := methodToEnum(method)
	if methodIndex == 255 {
		return nil
	}

	h := r.search(r.root, methodIndex, path, 0, params)
	if h == nil {
		params.reset()
	}

	return h
}

func (r *Router) Remove(method string, path string) {
	panic("unimplemented")
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
