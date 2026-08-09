package gohttprouter

import (
	"context"
	"encoding/json"
	"net/http"
)

type Handler interface {
	Handle(c Context)
}

type Context struct {
	ctx    context.Context
	req    *http.Request
	params *Params
	w      http.ResponseWriter
}

func (c *Context) Write(status int, data []byte) (int, error) {
	return c.w.Write(data)
}

func (c *Context) WriteJSON(status int, v any) error {
	c.w.WriteHeader(status)
	return json.NewEncoder(c.w).Encode(v)
}

type methodHandler struct {
	get     Handler
	query   Handler
	post    Handler
	patch   Handler
	put     Handler
	delete  Handler
	connect Handler
	options Handler
	head    Handler
}

func (h *methodHandler) count() int {
	handlers := []Handler{
		h.get,
		h.query,
		h.post,
		h.patch,
		h.put,
		h.delete,
		h.connect,
		h.options,
		h.head,
	}

	n := 0

	for _, f := range handlers {
		if f != nil {
			n++
		}
	}

	return n
}

func (h *methodHandler) insertHandler(method string, handler Handler) {
	switch method {
	case http.MethodGet:
		h.get = handler
	case http.MethodPost:
		h.post = handler
	case http.MethodPatch:
		h.patch = handler
	case http.MethodPut:
		h.put = handler
	case http.MethodDelete:
		h.delete = handler
	case http.MethodConnect:
		h.connect = handler
	case http.MethodOptions:
		h.options = handler
	case http.MethodHead:
		h.head = handler
	}
}

func (h *methodHandler) getHandler(method string) Handler {
	switch method {
	case http.MethodGet:
		return h.get
	case http.MethodPost:
		return h.post
	case http.MethodPatch:
		return h.patch
	case http.MethodPut:
		return h.put
	case http.MethodDelete:
		return h.delete
	case http.MethodConnect:
		return h.connect
	case http.MethodOptions:
		return h.options
	case http.MethodHead:
		return h.head
	default:
		return nil
	}
}
