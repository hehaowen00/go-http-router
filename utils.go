package gohttprouter

import (
	"fmt"
	"strings"
)

type Handler interface {
	Handle(c Context)
}

type HandlerFunc func(c Context)

func (h HandlerFunc) Handle(c Context) {
	h(c)
}

func NewHandlerFunc(handler func(c Context)) Handler {
	return HandlerFunc(handler)
}

func isParam(segment string) bool {
	return len(segment) > 0 && segment[0] == ':'
}

func paramName(segment string) string {
	return strings.TrimPrefix(segment, ":")
}

func isCatchAll(segment string) bool {
	return len(segment) > 0 && segment[0] == '*'
}

func catchAllName(segment string) string {
	if len(segment) > 0 && segment[0] == '*' {
		return segment[1:]
	}

	return segment
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
		if isParam(v) || isCatchAll(v) {
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

		if k[0] != ':' && k[0] != '*' {
			continue
		}

		if k[0] == '*' && i != len(xs)-1 {
			return fmt.Errorf("catch all segment must be the last segment")
		}

		if k == ":" || k == "*" {
			return fmt.Errorf("param name cannot be empty - %s", k)
		}

		_, ok := set[k]
		if ok {
			return fmt.Errorf("duplicate param name - %s", k)
		}

		set[k] = struct{}{}
	}

	if len(set) > maxParams {
		return fmt.Errorf("wildcard limit exceeded")
	}

	return nil
}
