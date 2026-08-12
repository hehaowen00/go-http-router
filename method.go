package gohttprouter

import "net/http"

type methodEnum int

const (
	methodGet methodEnum = iota
	methodQuery
	methodPost
	methodPatch
	methodPut
	methodDelete
	methodConnect
	methodOptions
	methodHead
	methodCount
)

func methodToEnum(method string) methodEnum {
	switch method {
	case http.MethodGet:
		return methodGet
	case "QUERY":
		return methodQuery
	case http.MethodPost:
		return methodPost
	case http.MethodPatch:
		return methodPatch
	case http.MethodPut:
		return methodPut
	case http.MethodDelete:
		return methodDelete
	case http.MethodConnect:
		return methodConnect
	case http.MethodOptions:
		return methodOptions
	case http.MethodHead:
		return methodHead
	default:
		return 255
	}
}

type methodMask uint16

type methodHandler struct {
	handlers [9]Handler
	count    int
}

func (h *methodHandler) Len() int {
	return h.count
}

func (h *methodHandler) Get(method methodEnum) Handler {
	return h.handlers[method]
}

func (h *methodHandler) Insert(method methodEnum, handler Handler) bool {
	if h.handlers[method] == nil {
		h.count++
		h.handlers[method] = handler
		return true
	}

	h.handlers[method] = handler

	return false
}

func (h *methodHandler) Remove(method methodEnum) bool {
	v := h.handlers[method] != nil
	if v {
		h.count--
	}

	h.handlers[method] = nil

	return v
}
