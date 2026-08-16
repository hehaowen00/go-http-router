package gohttprouter

import "net/http"

const queryMethod = "QUERY"

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
	methodNotFound = 255
)

func methodToEnum(method string) methodEnum {
	switch method {
	case http.MethodGet:
		return methodGet
	case queryMethod:
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
		return methodNotFound
	}
}
