package gohttprouter

import (
	"net/http"
	"strings"
)

type Router struct {
	root *node
}

func New() *Router {
	root := newNode()

	return &Router{
		root: root,
	}
}

func (r *Router) Add(method string, path string, handler Handler) {
	pathSeq := splitPath(path)

	err := validateSeq(pathSeq)
	if err != nil {
		panic(err)
	}

	r.root.insert(method, pathSeq, handler)
}

func (r *Router) Search(method string, path string, params *Params) Handler {
	params.clear()

	if !strings.HasSuffix(path, "/") {
		path += "/"
	}

	// path = strings.TrimPrefix(path, "/")

	path = strings.ReplaceAll(path, "//", "/")
	path = strings.ReplaceAll(path, "//", "/")

	h := r.root.search(method, path, params)
	if h == nil {
		params.clear()
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
