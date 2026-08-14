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

	if len(path) == 0 {
		return []string{"/"}
	}

	if path[0] != '/' {
		path = "/" + path
	}

	if strings.Contains(path, "//") {
		path = collapseSlashes(path)
	}

	i := 1
	end := len(path)

	for end > i && path[end-1] == '/' {
		end--
	}

	if i == end {
		return []string{"/"}
	}

	res := make([]string, 0, 4)
	staticStart := -1

	for i < end {
		if path[i] == '/' {
			i++
			continue
		}

		segStart := i
		i = nextSlash(path, i, end)
		seg := path[segStart:i]

		if isParam(seg) || isCatchAll(seg) {
			if staticStart >= 0 {
				res = append(res, path[staticStart:segStart])
				staticStart = -1
			}

			res = append(res, seg)
			continue
		}

		if staticStart < 0 {
			staticStart = segStart - 1
		}
	}

	if staticStart >= 0 {
		res = append(res, path[staticStart:end]+"/")
	}

	return res
}

func collapseSlashes(s string) string {
	b := make([]byte, 0, len(s))

	last := false

	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			if last {
				continue
			}

			last = true
		} else {
			last = false
		}

		b = append(b, s[i])
	}

	return string(b)
}

func validateSeq(xs []string) error {
	set := [32]string{}
	idx := 0

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

		for i := range idx {
			if set[i] == k {
				return fmt.Errorf("duplicate param name - %s", k)
			}
		}

		set[idx] = k
		idx++
	}

	if idx > maxParams {
		return fmt.Errorf("wildcard limit exceeded")
	}

	return nil
}
