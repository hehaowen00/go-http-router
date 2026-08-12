package gohttprouter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

func isParam(segment string) bool {
	return strings.HasPrefix(segment, ":")
}

func paramName(segment string) string {
	return strings.TrimPrefix(segment, ":")
}

func longestMatch(left, right string) int {
	var i int

	l := min(len(left), len(right))

	for i = range l {
		if left[i] != right[i] {
			return i
		}
	}

	return l
}

func splitPath(path string) []string {
	path = strings.TrimSpace(path)

	for strings.HasPrefix(path, "/") {
		path = strings.TrimPrefix(path, "/")
	}

	for strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}

	if path == "" {
		return []string{"/"}
	}

	path = strings.ReplaceAll(path, "//", "/")
	xs := strings.Split(path, "/")

	var buf string
	var res []string

	for _, v := range xs {
		if strings.HasPrefix(v, ":") {
			if len(buf) > 0 {
				res = append(res, buf+"/")
				buf = ""
			}
			res = append(res, v)
		} else {
			buf += "/" + v
		}
	}

	if buf != "" {
		res = append(res, buf+"/")
	}

	return res
}

func validateSeq(xs []string) error {
	set := map[string]struct{}{}

	for i := range xs {
		k := xs[i]
		if k[0] != ':' {
			continue
		}

		_, ok := set[k]
		if ok {
			return fmt.Errorf("duplicate param name - %s", k)
		}

		set[k] = struct{}{}
	}

	if len(set) > 32 {
		return fmt.Errorf("wildcard limit exceeded")
	}

	return nil
}
