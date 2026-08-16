package gohttprouter

import (
	"fmt"
	"strings"
)

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
	return strings.TrimPrefix(segment, "*")
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

		for j := range idx {
			if set[j] == k {
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

func staticKey(s string) string {
	n := len(s)
	if n <= 1 {
		return s
	}

	if s[n-1] != '/' {
		return s
	}

	return s[:n-1]
}
